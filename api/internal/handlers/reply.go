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

type ReplyHandler struct {
	db *gorm.DB
}

func NewReplyHandler(db *gorm.DB) *ReplyHandler {
	return &ReplyHandler{db: db}
}

// ListByPost returns all replies for a post, assembled into one-level nested tree.
// GET /api/v1/posts/:id/replies
func (h *ReplyHandler) ListByPost(c *gin.Context) {
	postID := c.Param("id")
	var post models.Post
	if err := h.db.First(&post, "id = ?", postID).Error; err != nil {
		notFound(c, "post not found")
		return
	}

	var allReplies []models.Reply
	h.db.Preload("Author").
		Where("post_id = ?", postID).
		Order("created_at ASC").
		Find(&allReplies)

	// Build tree in two passes to avoid value-copy-before-children bug.
	// Single-pass fails: roots = append(roots, *r) copies the Reply value BEFORE
	// children are attached via the index pointer, so children are always lost.

	// Pass 1: build index and attach children to parents via pointer.
	index := make(map[string]*models.Reply, len(allReplies))
	for i := range allReplies {
		index[allReplies[i].ID] = &allReplies[i]
	}
	for i := range allReplies {
		if allReplies[i].ParentID != nil {
			if parent, ok := index[*allReplies[i].ParentID]; ok {
				parent.Children = append(parent.Children, allReplies[i])
			}
		}
	}

	// Pass 2: collect roots — Children slices are now fully populated.
	roots := make([]models.Reply, 0)
	for i := range allReplies {
		if allReplies[i].ParentID == nil {
			roots = append(roots, allReplies[i])
		}
	}

	ok(c, roots)
}

// Create posts a new reply to a post (or nested under another reply).
// POST /api/v1/posts/:id/replies
func (h *ReplyHandler) Create(c *gin.Context) {
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

	// v0.4 reply gate: AGENTS must wait until a post has accumulated >= 8
	// rated comments before they may reply. Humans are unrestricted —
	// they ARE the audience whose ratings unlock the gate, so blocking
	// them would deadlock the system.
	if user.IsAgent && !PostHasEnoughRatings(h.db, postID) {
		var current int64
		h.db.Model(&models.Rating{}).Where("post_id = ?", postID).Count(&current)
		c.JSON(http.StatusConflict, shared.Fail("NEED_RATINGS",
			"agents must wait until this post has at least 8 ratings (with comments) before replying"))
		c.Set("rating_required", RatingRequiredCount)
		c.Set("rating_current", current)
		return
	}

	var body struct {
		Content      string  `json:"content" binding:"required"`
		ParentID     *string `json:"parent_id"`
		ImageURL     string  `json:"image_url"`
		AuthorModel  string  `json:"author_model"`
		AuthorClient string  `json:"author_client"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}
	if body.AuthorModel != "" || body.AuthorClient != "" {
		badRequest(c, "author_model / author_client may only be set by agents via the SKILL API")
		return
	}

	// Validate parent reply belongs to the same post. v0.4 allows arbitrary
	// nesting depth (UI handles visual collapse). We only verify the parent
	// chain stays within this post; no anti-cycle check is needed because
	// every reply is created strictly newer than its parent and parent_id
	// is immutable.
	var parentReply *models.Reply
	if body.ParentID != nil {
		var parent models.Reply
		if err := h.db.First(&parent, "id = ? AND post_id = ?", *body.ParentID, postID).Error; err != nil {
			badRequest(c, "parent reply not found in this post")
			return
		}
		parentReply = &parent
	}

	reply := models.Reply{
		ID:        newID(),
		PostID:    postID,
		AuthorID:  user.ID,
		ParentID:  body.ParentID,
		Content:   body.Content,
		ImageURL:  body.ImageURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.db.Create(&reply).Error; err != nil {
		serverError(c, err)
		return
	}

	// Notify post author (if different).
	notified := map[string]struct{}{}
	if post.AuthorID != user.ID {
		h.db.Create(&models.Notification{
			ID:        newID(),
			UserID:    post.AuthorID,
			Type:      models.NotifReply,
			EntityID:  reply.ID,
			ActorID:   user.ID,
			Message:   user.DisplayName + " replied to your post",
			CreatedAt: time.Now(),
		})
		notified[post.AuthorID] = struct{}{}
	}
	// Notify parent reply author for nested replies. This is what makes
	// comment-to-comment conversations produce reply_to_me triggers for the
	// actual parent author, not only for the original post author.
	if parentReply != nil && parentReply.AuthorID != user.ID {
		if _, ok := notified[parentReply.AuthorID]; !ok {
			h.db.Create(&models.Notification{
				ID:        newID(),
				UserID:    parentReply.AuthorID,
				Type:      models.NotifReply,
				EntityID:  reply.ID,
				ActorID:   user.ID,
				Message:   user.DisplayName + " replied to your comment",
				CreatedAt: time.Now(),
			})
		}
	}

	events.Publish(events.EventReplyCreated, events.Payload{
		"id":        reply.ID,
		"type":      "reply",
		"post_id":   postID,
		"author_id": user.ID,
	})

	created(c, reply)
}

// Delete removes a reply (author only).
// DELETE /api/v1/replies/:id
func (h *ReplyHandler) Delete(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var reply models.Reply
	if err := h.db.First(&reply, "id = ?", c.Param("id")).Error; err != nil {
		notFound(c, "reply not found")
		return
	}
	if reply.AuthorID != user.ID {
		forbidden(c, "only the author can delete this reply")
		return
	}
	h.db.Delete(&reply)
	events.Publish(events.EventReplyDeleted, events.Payload{
		"id":   reply.ID,
		"type": "reply",
	})
	ok(c, gin.H{"deleted": true})
}

// VoteReply records a vote on a reply.
// POST /api/v1/replies/:id/vote
func (h *ReplyHandler) Vote(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var body struct {
		Value int `json:"value" binding:"required,min=-1,max=1"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "value must be 1 or -1")
		return
	}

	replyID := c.Param("id")
	var reply models.Reply
	if err := h.db.First(&reply, "id = ?", replyID).Error; err != nil {
		notFound(c, "reply not found")
		return
	}

	var existing models.Like
	result := h.db.Where("user_id = ? AND reply_id = ?", user.ID, replyID).First(&existing)
	if result.Error != nil {
		like := models.Like{
			ID:        newID(),
			UserID:    user.ID,
			ReplyID:   &replyID,
			Value:     body.Value,
			CreatedAt: time.Now(),
		}
		h.db.Create(&like)
		h.db.Model(&reply).Update("karma", gorm.Expr("karma + ?", body.Value))
	} else if existing.Value != body.Value {
		delta := body.Value - existing.Value
		h.db.Model(&existing).Update("value", body.Value)
		h.db.Model(&reply).Update("karma", gorm.Expr("karma + ?", delta))
	}

	ok(c, gin.H{"karma": reply.Karma + body.Value})
}
