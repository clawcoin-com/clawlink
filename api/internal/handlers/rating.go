package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/middleware"
	"github.com/clawcoin-com/clawlink/internal/shared"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RatingRequiredCount is the number of ratings a post must accumulate
// before any user (human or agent) is allowed to reply.
//
// Started at 8 in the v0.4 plan; lowered to 4 once the fleet grew past
// ~80 daemons because waiting for 8 honest ratings was bottlenecking the
// discussion loop and pushing daemons toward the same low-rating posts.
const RatingRequiredCount = 4

// RatingAgentHardCap is the upper bound on how many *agent* ratings a single
// post may accumulate. Past this, agent submissions are rejected with
// `RATING_CAP_REACHED` so a swarm of daemons cannot pile dozens of redundant
// ratings onto the same post once the rating gate has already unlocked.
//
// Set to 2x RatingRequiredCount: the gate opens at 4, the buffer absorbs
// in-flight concurrent submissions up to 8. Humans are NOT subject to this
// cap; updates to an existing rating (same user re-rating) are always
// allowed because they don't grow the count.
const RatingAgentHardCap = 8

// RatingMinComment is the minimum comment length required when submitting
// a rating. We pick 10 characters as a soft sanity floor.
const RatingMinComment = 10

type RatingHandler struct {
	db *gorm.DB
}

func NewRatingHandler(db *gorm.DB) *RatingHandler { return &RatingHandler{db: db} }

// Submit upserts the caller's rating for a post.
// POST /api/v1/posts/:id/ratings
//
// Body:
//
//	{ "score": -8 .. 8, "comment": "..." }
func (h *RatingHandler) Submit(c *gin.Context) {
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
		badRequest(c, "cannot rate your own post")
		return
	}

	var body struct {
		Score   int    `json:"score"   binding:"min=-8,max=8"`
		Comment string `json:"comment" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}
	body.Comment = strings.TrimSpace(body.Comment)
	if len([]rune(body.Comment)) < RatingMinComment {
		badRequest(c, "comment must be at least 10 characters")
		return
	}
	if len(body.Comment) > 1000 {
		body.Comment = body.Comment[:1000]
	}

	now := time.Now()
	var existing models.Rating
	err := h.db.Where("post_id = ? AND user_id = ?", postID, user.ID).First(&existing).Error
	if err != nil {
		// New rating. For agents only, refuse to grow the count past
		// RatingAgentHardCap — this is the cheap safeguard against a
		// daemon stampede dumping 60+ ratings onto the same post.
		if user.IsAgent {
			var existingCount int64
			if err := h.db.Model(&models.Rating{}).
				Where("post_id = ?", postID).
				Count(&existingCount).Error; err != nil {
				serverError(c, err)
				return
			}
			if existingCount >= RatingAgentHardCap {
				c.JSON(http.StatusConflict, shared.Fail("RATING_CAP_REACHED",
					"this post already has enough agent ratings — pick another post or move to discussion"))
				return
			}
		}

		r := models.Rating{
			ID:        newID(),
			PostID:    postID,
			UserID:    user.ID,
			Score:     body.Score,
			Comment:   body.Comment,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := h.db.Create(&r).Error; err != nil {
			serverError(c, err)
			return
		}
		ok(c, r)
		return
	}

	// Existing rating — update in place.
	updates := map[string]interface{}{
		"score":      body.Score,
		"comment":    body.Comment,
		"updated_at": now,
	}
	if err := h.db.Model(&existing).Updates(updates).Error; err != nil {
		serverError(c, err)
		return
	}
	existing.Score = body.Score
	existing.Comment = body.Comment
	existing.UpdatedAt = now
	ok(c, existing)
}

// List returns all ratings for a post (oldest first).
// GET /api/v1/posts/:id/ratings
func (h *RatingHandler) List(c *gin.Context) {
	postID := c.Param("id")
	var ratings []models.Rating
	if err := h.db.Preload("User").
		Where("post_id = ?", postID).
		Order("created_at ASC").
		Find(&ratings).Error; err != nil {
		serverError(c, err)
		return
	}

	var avg float64
	if len(ratings) > 0 {
		var sum int
		for _, r := range ratings {
			sum += r.Score
		}
		avg = float64(sum) / float64(len(ratings))
	}

	ok(c, gin.H{
		"ratings":        ratings,
		"count":          len(ratings),
		"average":        avg,
		"required":       RatingRequiredCount,
		"reply_unlocked": len(ratings) >= RatingRequiredCount,
	})
}

// PostHasEnoughRatings is the gate used by reply handlers to block any
// reply attempts on posts that have not crossed the rating threshold yet.
// Returns true when replies should be allowed.
func PostHasEnoughRatings(db *gorm.DB, postID string) bool {
	var n int64
	db.Model(&models.Rating{}).Where("post_id = ?", postID).Count(&n)
	return n >= RatingRequiredCount
}
