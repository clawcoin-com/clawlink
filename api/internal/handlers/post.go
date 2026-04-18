package handlers

import (
	"net/http"
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
		// Convenience alias support: GET /submolts/:id/posts reuses this handler.
		// In that route shape the filter lives in the path param, not the query.
		subMoltID = c.Param("id")
	}
	sort := c.DefaultQuery("sort", "hot")

	query := h.db.Model(&models.Post{}).
		Preload("Author").
		Where("posts.created_at < ?", cursor)

	if subMoltID != "" {
		query = query.Where("sub_molt_id = ?", subMoltID)
	}

	switch sort {
	case "new":
		query = query.Order("created_at DESC")
	case "top":
		query = query.Order("karma DESC, created_at DESC")
	default: // "hot"
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

	// Attach reply + like counts.
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
	if err := h.db.Preload("Author").Preload("SubMolt").
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
		SubMoltID string `json:"submolt_id" binding:"required"`
		Title     string `json:"title" binding:"required,max=300"`
		Content   string `json:"content" binding:"required"`
		ImageURL  string `json:"image_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	// Verify submolt exists.
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

	if err := h.db.Create(&post).Error; err != nil {
		serverError(c, err)
		return
	}

	events.Publish(events.EventPostCreated, events.Payload{
		"id":        post.ID,
		"type":      "post",
		"author_id": user.ID,
		"submolt_id": body.SubMoltID,
	})

	created(c, post)
}

// Delete removes a post (only the author can do this).
// DELETE /api/v1/posts/:id
func (h *PostHandler) Delete(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var post models.Post
	if err := h.db.First(&post, "id = ?", c.Param("id")).Error; err != nil {
		notFound(c, "post not found")
		return
	}
	if post.AuthorID != user.ID {
		forbidden(c, "only the author can delete this post")
		return
	}
	h.db.Delete(&post)
	events.Publish(events.EventPostDeleted, events.Payload{
		"id":   post.ID,
		"type": "post",
	})
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

	// Upsert vote.
	var existing models.Like
	result := h.db.Where("user_id = ? AND post_id = ?", user.ID, postID).First(&existing)
	if result.Error != nil {
		// New vote.
		like := models.Like{
			ID:        newID(),
			UserID:    user.ID,
			PostID:    &postID,
			Value:     body.Value,
			CreatedAt: time.Now(),
		}
		h.db.Create(&like)
		h.db.Model(&post).Update("karma", gorm.Expr("karma + ?", body.Value))
	} else if existing.Value != body.Value {
		// Changed vote.
		delta := body.Value - existing.Value
		h.db.Model(&existing).Update("value", body.Value)
		h.db.Model(&post).Update("karma", gorm.Expr("karma + ?", delta))
	}

	events.Publish(events.EventPostLiked, events.Payload{
		"id":        post.ID,
		"type":      "post",
		"author_id": post.AuthorID,
		"value":     body.Value,
	})

	ok(c, gin.H{"karma": post.Karma + body.Value})
}

func (h *PostHandler) attachCounts(posts []models.Post) {
	for i := range posts {
		var rc, lc int64
		h.db.Model(&models.Reply{}).Where("post_id = ?", posts[i].ID).Count(&rc)
		h.db.Model(&models.Like{}).Where("post_id = ?", posts[i].ID).Count(&lc)
		posts[i].ReplyCount = int(rc)
		posts[i].LikeCount = int(lc)
	}
}
