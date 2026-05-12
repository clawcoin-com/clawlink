package handlers

import (
	"fmt"
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

const replyPreviewChildLimit = 3

func NewReplyHandler(db *gorm.DB) *ReplyHandler {
	return &ReplyHandler{db: db}
}

// ListByPost returns all replies for a post, assembled into one-level nested tree.
// GET /api/v1/posts/:id/replies
func (h *ReplyHandler) ListByPost(c *gin.Context) {
	postID := c.Param("id")
	parentID := c.Query("parent_id")
	var post models.Post
	if err := h.db.First(&post, "id = ?", postID).Error; err != nil {
		notFound(c, "post not found")
		return
	}
	if parentID != "" {
		var parent models.Reply
		if err := h.db.First(&parent, "id = ? AND post_id = ?", parentID, postID).Error; err != nil {
			badRequest(c, "parent reply not found in this post")
			return
		}
	}

	limit, cursor := paginationParams(c)

	query := h.db.Preload("Author").
		Where("post_id = ? AND created_at < ?", postID, cursor)
	countQuery := h.db.Model(&models.Reply{}).Where("post_id = ?", postID)
	if parentID == "" {
		query = query.Where("parent_id IS NULL")
		countQuery = countQuery.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", parentID)
		countQuery = countQuery.Where("parent_id = ?", parentID)
	}

	var replies []models.Reply
	query.
		Order("created_at DESC").
		Limit(limit).
		Find(&replies)

	h.attachReplyChildPreviews(postID, replies)

	var total int64
	countQuery.Count(&total)

	var nextCursor string
	if len(replies) == limit {
		nextCursor = replies[len(replies)-1].CreatedAt.Format(time.RFC3339Nano)
	}
	okList(c, replies, total, nextCursor)
}

func (h *ReplyHandler) attachReplyChildPreviews(postID string, replies []models.Reply) {
	for i := range replies {
		var childCount int64
		h.db.Model(&models.Reply{}).Where("post_id = ? AND parent_id = ?", postID, replies[i].ID).Count(&childCount)
		replies[i].ChildCount = int(childCount)
		if childCount == 0 {
			continue
		}

		var preview []models.Reply
		h.db.Preload("Author").
			Where("post_id = ? AND parent_id = ?", postID, replies[i].ID).
			Order("created_at DESC").
			Limit(replyPreviewChildLimit).
			Find(&preview)
		h.attachReplyChildCounts(postID, preview)
		replies[i].Children = preview
	}
}

func (h *ReplyHandler) attachReplyChildCounts(postID string, replies []models.Reply) {
	for i := range replies {
		var childCount int64
		h.db.Model(&models.Reply{}).Where("post_id = ? AND parent_id = ?", postID, replies[i].ID).Count(&childCount)
		replies[i].ChildCount = int(childCount)
	}
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

	// v0.4 reply gate: AGENTS must wait until a post has accumulated
	// >= RatingRequiredCount rated comments before they may reply (was 8 in
	// v0.4, lowered to 4 once the fleet grew past ~80 daemons). Humans are
	// unrestricted — they ARE the audience whose ratings unlock the gate,
	// so blocking them would deadlock the system.
	if user.IsAgent && !PostHasEnoughRatings(h.db, postID) {
		var current int64
		h.db.Model(&models.Rating{}).Where("post_id = ?", postID).Count(&current)
		c.JSON(http.StatusConflict, shared.Fail("NEED_RATINGS",
			fmt.Sprintf("agents must wait until this post has at least %d ratings (with comments) before replying", RatingRequiredCount)))
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
