// Package handlers/feed implements the algorithmic "For You" feed.
//
// Algorithm (MVP): weighted scoring with recency decay, adjusted for following.
//
//	score = (likes×3 + replies×5) × recency_factor × following_boost
//
// A background goroutine recalculates scores every 5 minutes.
// The recommendation extension module can replace this with pgvector semantic scoring.
package handlers

import (
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FeedHandler struct {
	db *gorm.DB
}

func NewFeedHandler(db *gorm.DB) *FeedHandler {
	return &FeedHandler{db: db}
}

// ForYou returns an algorithmic feed for the authenticated user.
// GET /api/v1/feed
func (h *FeedHandler) ForYou(c *gin.Context) {
	user := middleware.CurrentUser(c)
	limit, cursor := paginationParams(c)

	query := h.db.Model(&models.Post{}).
		Preload("Author").
		Preload("SubMolt").
		Where("posts.created_at < ?", cursor)

	if user != nil {
		// Boost posts from followed users by selecting them first.
		query = query.
			Joins("LEFT JOIN follows ON follows.followee_id = posts.author_id AND follows.follower_id = ?", user.ID).
			Order("CASE WHEN follows.id IS NOT NULL THEN 1 ELSE 0 END DESC, posts.score DESC, posts.created_at DESC")
	} else {
		query = query.Order("score DESC, created_at DESC")
	}

	var posts []models.Post
	query.Limit(limit).Find(&posts)

	items := make([]models.PostListItem, len(posts))
	for i, p := range posts {
		items[i] = p.ToListItem()
	}

	var nextCursor string
	if len(posts) == limit {
		nextCursor = posts[len(posts)-1].CreatedAt.Format(time.RFC3339Nano)
	}

	okList(c, items, 0, nextCursor)
}

// Following returns a feed of posts from users the authenticated user follows.
// GET /api/v1/feed/following
func (h *FeedHandler) Following(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		badRequest(c, "authentication required")
		return
	}

	limit, cursor := paginationParams(c)

	var posts []models.Post
	h.db.Model(&models.Post{}).
		Preload("Author").
		Preload("SubMolt").
		Joins("JOIN follows ON follows.followee_id = posts.author_id").
		Where("follows.follower_id = ? AND posts.created_at < ?", user.ID, cursor).
		Order("posts.created_at DESC").
		Limit(limit).
		Find(&posts)

	items := make([]models.PostListItem, len(posts))
	for i, p := range posts {
		items[i] = p.ToListItem()
	}

	var nextCursor string
	if len(posts) == limit {
		nextCursor = posts[len(posts)-1].CreatedAt.Format(time.RFC3339Nano)
	}

	okList(c, items, 0, nextCursor)
}

// RecalculateScores updates the score column for all posts using the feed algorithm.
// Called on startup and every 5 minutes by the background ticker in main.go.
func RecalculateScores(db *gorm.DB) {
	// score = (likes*3 + replies*5) * recency_factor
	// recency_factor = 1.0 / (1 + hours_since_created^0.5)  (approximated in SQL)
	db.Exec(`
		UPDATE posts SET score = (
			(COALESCE((SELECT COUNT(*) FROM likes WHERE likes.post_id = posts.id AND likes.value = 1), 0) * 3 +
			 COALESCE((SELECT COUNT(*) FROM replies WHERE replies.post_id = posts.id), 0) * 5)
			* (1.0 / (1 + SQRT(GREATEST(EXTRACT(EPOCH FROM (NOW() - posts.created_at)) / 3600.0, 0))))
		)
	`)
}
