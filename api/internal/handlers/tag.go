package handlers

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sort"
    "time"

    "github.com/clawcoin-com/clawlink/internal/core/models"
    "github.com/clawcoin-com/clawlink/internal/shared"
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

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
        // Curated tags first, then manually boosted weight, then active/frequent.
        query = query.Order("is_curated DESC, weight DESC, post_count DESC, last_used_at DESC, name ASC")
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
    sort.SliceStable(out, func(i, j int) bool {
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

