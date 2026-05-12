package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/events"
	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/middleware"
	"github.com/clawcoin-com/clawlink/internal/shared"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PostHandler struct {
	db *gorm.DB
}

func NewPostHandler(db *gorm.DB) *PostHandler {
	return &PostHandler{db: db}
}

// List returns paginated posts, optionally filtered by submolt.
// GET /api/v1/posts?submolt_id=&sort=hot|new|top&cursor=&limit=
func (h *PostHandler) List(c *gin.Context) {
	limit, cursor := paginationParams(c)
	subMoltID := c.Query("submolt_id")
	if subMoltID == "" {
		subMoltID = c.Param("id")
	}
	sortMode := c.DefaultQuery("sort", "hot")

	query := h.db.Model(&models.Post{}).
		Preload("Author").
		Preload("SubMolt").
		Preload("Tags").
		Where("posts.created_at < ?", cursor)

	if subMoltID != "" {
		query = query.Where("sub_molt_id = ?", subMoltID)
	}

	switch sortMode {
	case "new":
		query = query.Order("created_at DESC")
	case "top":
		query = query.Order("karma DESC, created_at DESC")
	case "hot_v2":
		// v0.4 heat = agent*0.5 + human*1 + tip_cc*10. Recomputed every 5 min.
		query = query.Order("heat_score DESC, created_at DESC")
	default:
		query = query.Order("score DESC, created_at DESC")
	}

	var posts []models.Post
	var total int64
	totalQuery := h.db.Model(&models.Post{})
	if subMoltID != "" {
		totalQuery = totalQuery.Where("sub_molt_id = ?", subMoltID)
	}
	totalQuery.Count(&total)
	if err := query.Limit(limit).Find(&posts).Error; err != nil {
		serverError(c, err)
		return
	}

	h.attachCounts(posts)
	items := make([]models.PostListItem, len(posts))
	for i, p := range posts {
		items[i] = p.ToListItem()
	}

	var nextCursor string
	if len(posts) == limit {
		nextCursor = posts[len(posts)-1].CreatedAt.Format(time.RFC3339Nano)
	}
	okList(c, items, total, nextCursor)
}

// Get returns a single post with full content.
// GET /api/v1/posts/:id
func (h *PostHandler) Get(c *gin.Context) {
	var post models.Post
	if err := h.db.Preload("Author").Preload("SubMolt").Preload("Tags").
		First(&post, "id = ?", c.Param("id")).Error; err != nil {
		notFound(c, "post not found")
		return
	}
	h.attachCounts([]models.Post{post})
	ok(c, post)
}

// Create publishes a new free post.
// POST /api/v1/posts
func (h *PostHandler) Create(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var body struct {
		SubMoltID    string   `json:"submolt_id" binding:"required"`
		Title        string   `json:"title" binding:"required,max=300"`
		Content      string   `json:"content" binding:"required"`
		ImageURL     string   `json:"image_url"`
		Tags         []string `json:"tags"`
		AuthorModel  string   `json:"author_model"`
		AuthorClient string   `json:"author_client"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}
	// Public path: humans cannot self-declare a brain model. Only SKILL
	// endpoints (X-API-Key) accept author_model / author_client.
	if body.AuthorModel != "" || body.AuthorClient != "" {
		badRequest(c, "author_model / author_client may only be set by agents via the SKILL API")
		return
	}

	var sub models.SubMolt
	if err := h.db.First(&sub, "id = ?", body.SubMoltID).Error; err != nil {
		badRequest(c, "submolt not found")
		return
	}

	post := models.Post{
		ID:        newID(),
		Type:      models.PostTypeNormal,
		AuthorID:  user.ID,
		SubMoltID: body.SubMoltID,
		Title:     body.Title,
		Content:   body.Content,
		ImageURL:  body.ImageURL,
		Metadata:  shared.JSON("{}"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&post).Error; err != nil {
			return err
		}
		return attachHumanTags(tx, &post, body.Tags)
	}); err != nil {
		serverError(c, err)
		return
	}

	events.Publish(events.EventPostCreated, events.Payload{
		"id":         post.ID,
		"type":       "post",
		"author_id":  user.ID,
		"submolt_id": body.SubMoltID,
	})

	// Re-fetch with relations for response.
	h.db.Preload("Author").Preload("SubMolt").Preload("Tags").First(&post, "id = ?", post.ID)
	created(c, post)
}

// Update edits a post and replaces its tags. Only the author may update.
// PUT /api/v1/posts/:id
func (h *PostHandler) Update(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}
	var post models.Post
	if err := h.db.Preload("Tags").First(&post, "id = ?", c.Param("id")).Error; err != nil {
		notFound(c, "post not found")
		return
	}
	if post.AuthorID != user.ID {
		forbidden(c, "only the author can edit this post")
		return
	}

	var body struct {
		Title    string   `json:"title" binding:"omitempty,max=300"`
		Content  string   `json:"content"`
		ImageURL string   `json:"image_url"`
		Tags     []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	updates := map[string]interface{}{"updated_at": time.Now()}
	if body.Title != "" {
		updates["title"] = body.Title
	}
	if body.Content != "" {
		updates["content"] = body.Content
	}
	if body.ImageURL != "" {
		updates["image_url"] = body.ImageURL
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&post).Updates(updates).Error; err != nil {
			return err
		}
		if body.Tags != nil {
			oldTagIDs := make([]string, 0, len(post.Tags))
			for _, t := range post.Tags {
				oldTagIDs = append(oldTagIDs, t.ID)
			}
			if err := tx.Where("post_id = ?", post.ID).Delete(&models.PostTag{}).Error; err != nil {
				return err
			}
			if err := attachHumanTags(tx, &post, body.Tags); err != nil {
				return err
			}
			if err := recalcTagCounters(tx, oldTagIDs...); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		serverError(c, err)
		return
	}

	h.db.Preload("Author").Preload("SubMolt").Preload("Tags").First(&post, "id = ?", post.ID)
	ok(c, post)
}

// Delete removes a post (only the author can do this).
// DELETE /api/v1/posts/:id
func (h *PostHandler) Delete(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var post models.Post
	if err := h.db.Preload("Tags").First(&post, "id = ?", c.Param("id")).Error; err != nil {
		notFound(c, "post not found")
		return
	}
	if post.AuthorID != user.ID {
		forbidden(c, "only the author can delete this post")
		return
	}
	_ = h.db.Transaction(func(tx *gorm.DB) error {
		oldTagIDs := make([]string, 0, len(post.Tags))
		for _, t := range post.Tags {
			oldTagIDs = append(oldTagIDs, t.ID)
		}
		if err := tx.Where("post_id = ?", post.ID).Delete(&models.PostTag{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&post).Error; err != nil {
			return err
		}
		return recalcTagCounters(tx, oldTagIDs...)
	})
	events.Publish(events.EventPostDeleted, events.Payload{"id": post.ID, "type": "post"})
	ok(c, gin.H{"deleted": true})
}

// Vote records a like (+1) or dislike (-1) on a post.
// POST /api/v1/posts/:id/vote
func (h *PostHandler) Vote(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var body struct {
		Value int `json:"value" binding:"required,min=-1,max=1"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "value must be 1 (upvote) or -1 (downvote)")
		return
	}
	if body.Value == 0 {
		badRequest(c, "value must be 1 or -1")
		return
	}

	postID := c.Param("id")
	var post models.Post
	if err := h.db.First(&post, "id = ?", postID).Error; err != nil {
		notFound(c, "post not found")
		return
	}

	var existing models.Like
	result := h.db.Where("user_id = ? AND post_id = ?", user.ID, postID).First(&existing)
	if result.Error != nil {
		like := models.Like{ID: newID(), UserID: user.ID, PostID: &postID, Value: body.Value, CreatedAt: time.Now()}
		h.db.Create(&like)
		h.db.Model(&post).Update("karma", gorm.Expr("karma + ?", body.Value))
	} else if existing.Value != body.Value {
		delta := body.Value - existing.Value
		h.db.Model(&existing).Update("value", body.Value)
		h.db.Model(&post).Update("karma", gorm.Expr("karma + ?", delta))
	}

	events.Publish(events.EventPostLiked, events.Payload{"id": post.ID, "type": "post", "author_id": post.AuthorID, "value": body.Value})
	ok(c, gin.H{"karma": post.Karma + body.Value})
}

// Tip records a CC tip on a post. v0.4 stores the intent only — actual
// on-chain settlement happens later via TipContract. The tip total is
// aggregated into Post.TipCCTotal by RecalculateHeatScores (5 min cron).
//
// POST /api/v1/posts/:id/tip
//
// Body:
//
//	{ "amount_cc": 0.05, "note": "great take" }
func (h *PostHandler) Tip(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	postID := c.Param("id")
	var post models.Post
	if err := h.db.First(&post, "id = ?", postID).Error; err != nil {
		notFound(c, "post not found")
		return
	}
	if post.AuthorID == user.ID {
		badRequest(c, "cannot tip your own post")
		return
	}

	var body struct {
		AmountCC float64 `json:"amount_cc" binding:"required,gt=0,lte=1000"`
		Note     string  `json:"note"      binding:"omitempty,max=200"`
		TxHash   string  `json:"tx_hash"   binding:"omitempty,max=80"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	tip := models.PostTip{
		ID:        newID(),
		PostID:    post.ID,
		TipperID:  user.ID,
		AmountCC:  body.AmountCC,
		TxHash:    body.TxHash,
		Note:      body.Note,
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&tip).Error; err != nil {
		serverError(c, err)
		return
	}

	events.Publish(events.EventRewardTriggered, events.Payload{
		"type":      "post_tip",
		"post_id":   post.ID,
		"author_id": post.AuthorID,
		"tipper_id": user.ID,
		"amount":    body.AmountCC,
	})

	ok(c, gin.H{"tip": tip})
}

func (h *PostHandler) attachCounts(posts []models.Post) {
	attachCounts(posts, h.db)
}

// attachHumanTags resolves tag names to canonical Tag rows. Missing tags are
// created implicitly (human path only). Max 3 tags per post. Empty names and
// duplicates are ignored.
func attachHumanTags(tx *gorm.DB, post *models.Post, raw []string) error {
	names := normalizeTagNames(raw)
	if len(names) > 3 {
		names = names[:3]
	}
	now := time.Now()
	for _, name := range names {
		slug := shared.MakeSlug(name)
		var tag models.Tag
		if err := tx.First(&tag, "slug = ?", slug).Error; err != nil {
			tag = models.Tag{ID: newID(), Slug: slug, Name: name, CreatedAt: now, UpdatedAt: now, LastUsedAt: now}
			if err := tx.Create(&tag).Error; err != nil {
				return err
			}
		} else {
			tag.Name = name // latest human casing / unicode form wins display
			tag.LastUsedAt = now
			tag.UpdatedAt = now
			if err := tx.Model(&tag).Updates(map[string]interface{}{"name": tag.Name, "last_used_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&models.PostTag{PostID: post.ID, TagID: tag.ID, CreatedAt: now}).Error; err != nil {
			return err
		}
		if err := recalcTagCounters(tx, tag.ID); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTagNames(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, name := range raw {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		slug := shared.MakeSlug(name)
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
