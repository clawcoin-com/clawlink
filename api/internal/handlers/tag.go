package handlers

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sort"
    "time"

    "github.com/clawcoin-com/clawlink/internal/core/config"
    "github.com/clawcoin-com/clawlink/internal/core/models"
    "github.com/clawcoin-com/clawlink/internal/middleware"
    "github.com/clawcoin-com/clawlink/internal/shared"
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

// paidTagDisabledMsg is the user-facing 503 body the gate returns. Kept as
// a constant so the wording matches between the two endpoints and tests
// can assert on it.
const paidTagDisabledMsg = "paid tag creation/promotion is not open yet — on-chain settlement lands with v0.5; set PAID_TAG_ENABLED=1 once it is live"

// paidTagsEnabled returns the runtime flag from config.App, defaulting to
// false (closed) when config has not been loaded for some reason. Belt-
// and-suspenders so a misconfigured deploy never accidentally accepts
// fake fees.
func paidTagsEnabled() bool {
    if config.App == nil {
        return false
    }
    return config.App.PaidTagEnabled
}

// middlewareCurrentUser is a tiny indirection so future tests can swap auth
// without touching every handler. Today it just forwards to middleware.
func middlewareCurrentUser(c *gin.Context) *models.User {
    return middleware.CurrentUser(c)
}

type TagHandler struct {
    db *gorm.DB
}

func NewTagHandler(db *gorm.DB) *TagHandler { return &TagHandler{db: db} }

// List returns all tags, optionally sorted. This is the public topic index.
// GET /api/v1/tags?sort=hot|new|alpha&limit=50
func (h *TagHandler) List(c *gin.Context) {
    limit := 50
    if raw := c.Query("limit"); raw != "" {
        if n, err := parsePositiveInt(raw, 1, 200); err == nil {
            limit = n
        }
    }
    sort := c.DefaultQuery("sort", "hot")

    query := h.db.Model(&models.Tag{})
    switch sort {
    case "new":
        query = query.Order("created_at DESC")
    case "alpha":
        query = query.Order("name ASC")
    default: // hot
        // Order: paid-promoted (active window) → curated → manually boosted
        // weight → active/frequent. The CASE expression converts the
        // boolean "promoted_now" into a sortable 0/1 we can DESC.
        query = query.Order("CASE WHEN paid_until > NOW() THEN 1 ELSE 0 END DESC, is_curated DESC, weight DESC, post_count DESC, last_used_at DESC, name ASC")
    }

    var tags []models.Tag
    if err := query.Limit(limit).Find(&tags).Error; err != nil {
        serverError(c, err)
        return
    }
    tags = mergeExternalTopics(tags)
    if len(tags) > limit {
        tags = tags[:limit]
    }
    c.JSON(http.StatusOK, shared.OK(tags))
}

// Get returns one tag by slug.
// GET /api/v1/tags/:slug
func (h *TagHandler) Get(c *gin.Context) {
    // External curated topic can exist even before any local tagged posts.
    for _, t := range fetchExternalTopics() {
        if t.Slug == c.Param("slug") {
            c.JSON(http.StatusOK, shared.OK(t))
            return
        }
    }
    var tag models.Tag
    if err := h.db.First(&tag, "slug = ?", c.Param("slug")).Error; err != nil {
        c.JSON(http.StatusNotFound, shared.Fail("NOT_FOUND", "tag not found"))
        return
    }
    c.JSON(http.StatusOK, shared.OK(tag))
}

// GetPosts lists posts under a specific tag.
// GET /api/v1/tags/:slug/posts?sort=hot|new|top&cursor=&limit=
func (h *TagHandler) GetPosts(c *gin.Context) {
    limit, cursor := paginationParams(c)
    sort := c.DefaultQuery("sort", "hot")

    var tag models.Tag
    if err := h.db.First(&tag, "slug = ?", c.Param("slug")).Error; err != nil {
        c.JSON(http.StatusNotFound, shared.Fail("NOT_FOUND", "tag not found"))
        return
    }

    query := h.db.Model(&models.Post{}).
        Joins("JOIN post_tags ON post_tags.post_id = posts.id").
        Where("post_tags.tag_id = ? AND posts.created_at < ?", tag.ID, cursor).
        Preload("Author").
        Preload("SubMolt").
        Preload("Tags")

    switch sort {
    case "new":
        query = query.Order("posts.created_at DESC")
    case "top":
        query = query.Order("posts.karma DESC, posts.created_at DESC")
    default:
        query = query.Order("posts.score DESC, posts.created_at DESC")
    }

    var posts []models.Post
    if err := query.Limit(limit).Find(&posts).Error; err != nil {
        serverError(c, err)
        return
    }

    // Attach counts same as PostHandler list path does.
    for i := range posts {
        var rc, lc int64
        h.db.Model(&models.Reply{}).Where("post_id = ?", posts[i].ID).Count(&rc)
        h.db.Model(&models.Like{}).Where("post_id = ?", posts[i].ID).Count(&lc)
        posts[i].ReplyCount = int(rc)
        posts[i].LikeCount = int(lc)
    }

    items := make([]models.PostListItem, len(posts))
    for i, p := range posts {
        items[i] = p.ToListItem()
    }

    var nextCursor string
    if len(posts) == limit {
        nextCursor = posts[len(posts)-1].CreatedAt.Format(time.RFC3339Nano)
    }

    // Total count for this tag.
    var total int64
    h.db.Model(&models.PostTag{}).Where("tag_id = ?", tag.ID).Count(&total)
    okList(c, items, total, nextCursor)
}

// TagCreateFeeCC is the v0.4 cost (in CC) to register a brand-new tag. Paid
// once, permanent ownership semantics will land later (v0.5+); for now it is
// a spam gate.
const TagCreateFeeCC = 0.05

// TagPromoteFeeCC is the v0.4 promotion price; one window = 24h.
const TagPromoteFeeCC = 1.0

// TagPromoteWindow is how long a single promotion lasts.
const TagPromoteWindow = 24 * time.Hour

// Create registers a new tag, charging TagCreateFeeCC to the caller.
// POST /api/v1/tags
//
// Body:
//
//	{ "name": "AI Agents", "description": "...", "tx_hash": "0x..." }
//
// v0.4 stores the tx hash but does not verify on-chain settlement; that
// belongs to TipContract once it is live. Tags created by curated sources
// (the external topic API) bypass this endpoint and are merged at read
// time, so this fee only applies to user-initiated tag registration.
//
// Until PAID_TAG_ENABLED flips true, this endpoint returns 503 with code
// NOT_IMPLEMENTED so the UI never has an excuse to pretend payment works.
// Implicit tag creation while posting (handlers/post.go.attachHumanTags)
// is unaffected — that path is free and always has been.
func (h *TagHandler) Create(c *gin.Context) {
    if !paidTagsEnabled() {
        c.JSON(http.StatusServiceUnavailable, shared.Fail("NOT_IMPLEMENTED", paidTagDisabledMsg))
        return
    }
    user := h.currentUser(c)
    if user == nil {
        c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
        return
    }

    var body struct {
        Name        string `json:"name"        binding:"required,min=2,max=80"`
        Description string `json:"description" binding:"omitempty,max=300"`
        TxHash      string `json:"tx_hash"     binding:"omitempty,max=80"`
    }
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, shared.Fail("BAD_REQUEST", err.Error()))
        return
    }

    slug := shared.MakeSlug(body.Name)
    var existing models.Tag
    if err := h.db.First(&existing, "slug = ?", slug).Error; err == nil {
        c.JSON(http.StatusConflict, shared.Fail("ALREADY_EXISTS",
            "tag already exists; only the registration of a new tag carries a fee"))
        return
    }

    now := time.Now()
    tag := models.Tag{
        ID:          newID(),
        Slug:        slug,
        Name:        body.Name,
        Description: body.Description,
        IsCurated:   false,
        Weight:      0,
        PostCount:   0,
        LastUsedAt:  now,
        CreatedAt:   now,
        UpdatedAt:   now,
    }
    payment := models.TagPayment{
        ID:        newID(),
        TagID:     tag.ID,
        PayerID:   user.ID,
        AmountCC:  TagCreateFeeCC,
        Reason:    models.TagPayRegister,
        TxHash:    body.TxHash,
        CreatedAt: now,
    }
    if err := h.db.Transaction(func(tx *gorm.DB) error {
        if err := tx.Create(&tag).Error; err != nil {
            return err
        }
        return tx.Create(&payment).Error
    }); err != nil {
        c.JSON(http.StatusInternalServerError, shared.Fail("DB_ERROR", err.Error()))
        return
    }

    c.JSON(http.StatusCreated, shared.OK(gin.H{
        "tag":     tag,
        "payment": payment,
        "fee_cc":  TagCreateFeeCC,
    }))
}

// Promote pays TagPromoteFeeCC to lift a tag into the promoted rail of the
// /tags listing for TagPromoteWindow.
// POST /api/v1/tags/:slug/promote
//
// Body:
//
//	{ "tx_hash": "0x..." }
//
// Each call extends PaidUntil by TagPromoteWindow from the current value
// (or now, if the previous window already lapsed).
//
// Gated by PAID_TAG_ENABLED, same reasoning as Create above.
func (h *TagHandler) Promote(c *gin.Context) {
    if !paidTagsEnabled() {
        c.JSON(http.StatusServiceUnavailable, shared.Fail("NOT_IMPLEMENTED", paidTagDisabledMsg))
        return
    }
    user := h.currentUser(c)
    if user == nil {
        c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
        return
    }

    var tag models.Tag
    if err := h.db.First(&tag, "slug = ?", c.Param("slug")).Error; err != nil {
        c.JSON(http.StatusNotFound, shared.Fail("NOT_FOUND", "tag not found"))
        return
    }

    var body struct {
        TxHash string `json:"tx_hash" binding:"omitempty,max=80"`
    }
    _ = c.ShouldBindJSON(&body)

    now := time.Now()
    base := now
    if tag.PaidUntil.After(now) {
        base = tag.PaidUntil
    }
    newUntil := base.Add(TagPromoteWindow)
    payment := models.TagPayment{
        ID:        newID(),
        TagID:     tag.ID,
        PayerID:   user.ID,
        AmountCC:  TagPromoteFeeCC,
        Reason:    models.TagPayPromote,
        TxHash:    body.TxHash,
        CreatedAt: now,
    }
    if err := h.db.Transaction(func(tx *gorm.DB) error {
        if err := tx.Model(&tag).Update("paid_until", newUntil).Error; err != nil {
            return err
        }
        return tx.Create(&payment).Error
    }); err != nil {
        c.JSON(http.StatusInternalServerError, shared.Fail("DB_ERROR", err.Error()))
        return
    }
    tag.PaidUntil = newUntil
    c.JSON(http.StatusOK, shared.OK(gin.H{
        "tag":         tag,
        "payment":     payment,
        "fee_cc":      TagPromoteFeeCC,
        "promoted_until": newUntil,
    }))
}

// currentUser is a small helper because tag.go currently lives in the same
// package as the AuthMiddleware-using handlers; we just call middleware.CurrentUser
// directly. Defined as a method for symmetry with handler styles.
func (h *TagHandler) currentUser(c *gin.Context) *models.User {
    return middlewareCurrentUser(c)
}

// parsePositiveInt parses s, clamps to [min,max]. Shared only in this file.
func parsePositiveInt(s string, min, max int) (int, error) {
    var n int
    if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
        return 0, err
    }
    if n < min { n = min }
    if n > max { n = max }
    return n, nil
}

type externalTopicsEnvelope struct {
    Topics []struct {
        ID          string `json:"id"`
        Title       string `json:"title"`
        Description string `json:"description"`
    } `json:"topics"`
}

// fetchExternalTopics loads the canonical topic seed list from ClawCoin testnet.
// Best-effort: failures return nil so the local tag index still works.
func fetchExternalTopics() []models.Tag {
    const url = "https://api-testnet.clawcoin.com/cc_bc/v1/qa/topics"
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Get(url)
    if err != nil {
        return nil
    }
    defer resp.Body.Close()
    raw, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil
    }
    var env externalTopicsEnvelope
    if err := json.Unmarshal(raw, &env); err != nil {
        return nil
    }
    out := make([]models.Tag, 0, len(env.Topics))
    now := time.Now()
    for _, t := range env.Topics {
        out = append(out, models.Tag{
            ID:          "external-topic-" + t.ID,
            Slug:        shared.MakeSlug(t.Title),
            Name:        t.Title,
            Description: t.Description,
            IsCurated:   true,
            Weight:      10000,
            LastUsedAt:  now,
            CreatedAt:   now,
            UpdatedAt:   now,
        })
    }
    return out
}

// mergeExternalTopics overlays external curated topics on top of local tags.
// If a local tag has the same slug, its post_count is preserved but the
// external title/description/curation metadata wins.
func mergeExternalTopics(local []models.Tag) []models.Tag {
    bySlug := map[string]models.Tag{}
    for _, t := range local {
        bySlug[t.Slug] = t
    }
    for _, ext := range fetchExternalTopics() {
        if cur, ok := bySlug[ext.Slug]; ok {
            cur.Name = ext.Name
            cur.Description = ext.Description
            cur.IsCurated = true
            if cur.Weight < ext.Weight {
                cur.Weight = ext.Weight
            }
            bySlug[ext.Slug] = cur
        } else {
            bySlug[ext.Slug] = ext
        }
    }
    out := make([]models.Tag, 0, len(bySlug))
    for _, t := range bySlug {
        out = append(out, t)
    }
    // Mirror the SQL ORDER BY in tag.List so the merged listing keeps the
    // same priority chain after we splice in external curated topics:
    //   paid promotion (active) → curated → manual weight → post_count → recency → name
    // Without the paid-first hop here, an active TagPayment loses to every
    // curated external topic — silently breaking the §4 "promoted goes
    // first" rule on any /tags?sort=hot listing that includes external
    // topics (which is most of them).
    now := time.Now()
    sort.SliceStable(out, func(i, j int) bool {
        pi := out[i].PaidUntil.After(now)
        pj := out[j].PaidUntil.After(now)
        if pi != pj {
            return pi
        }
        if out[i].IsCurated != out[j].IsCurated {
            return out[i].IsCurated
        }
        if out[i].Weight != out[j].Weight {
            return out[i].Weight > out[j].Weight
        }
        if out[i].PostCount != out[j].PostCount {
            return out[i].PostCount > out[j].PostCount
        }
        if !out[i].LastUsedAt.Equal(out[j].LastUsedAt) {
            return out[i].LastUsedAt.After(out[j].LastUsedAt)
        }
        return out[i].Name < out[j].Name
    })
    return out
}

