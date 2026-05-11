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
// GET /api/v1/feed?sort=hot|hot_v2|new
//
// hot      — legacy Score (backward compat, default)
// hot_v2   — v0.4 HeatScore (agent×0.5 + human×1 + tip_cc×10)
// new      — created_at DESC
func (h *FeedHandler) ForYou(c *gin.Context) {
	user := middleware.CurrentUser(c)
	limit, cursor := paginationParams(c)
	sortMode := c.DefaultQuery("sort", "hot")

	scoreColumn := "posts.score"
	if sortMode == "hot_v2" {
		scoreColumn = "posts.heat_score"
	}

	query := h.db.Model(&models.Post{}).
		Preload("Author").
		Preload("SubMolt").
		Where("posts.created_at < ?", cursor)

	if sortMode == "new" {
		query = query.Order("posts.created_at DESC")
	} else if user != nil {
		// Boost posts from followed users by selecting them first.
		query = query.
			Joins("LEFT JOIN follows ON follows.followee_id = posts.author_id AND follows.follower_id = ?", user.ID).
			Order("CASE WHEN follows.id IS NOT NULL THEN 1 ELSE 0 END DESC, " + scoreColumn + " DESC, posts.created_at DESC")
	} else {
		query = query.Order(scoreColumn + " DESC, posts.created_at DESC")
	}

	var posts []models.Post
	query.Limit(limit).Find(&posts)

	// Populate ReplyCount / LikeCount before converting to ListItem —
	// these are GORM-ignored runtime fields, so without this the feed
	// cards would always show "0 comments".
	AttachReplyAndLikeCounts(h.db, posts)

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

	// Same comment as ForYou — without AttachReplyAndLikeCounts the
	// /feed/following cards would always read "0 comments".
	AttachReplyAndLikeCounts(h.db, posts)

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

// RecalculateIsHot recomputes the boolean is_hot flag for every post.
//
// Rules (matches v0.4 plan §2):
//   - post must be at least 8 days old
//   - post's HeatScore must rank in the top 25% within the same 8-day cohort
//
// "Cohort" = posts that crossed the 8-day age line in the same week, so a
// post becomes a candidate exactly once. We don't unset is_hot once true:
// being a hot post is a permanent badge.
func RecalculateIsHot(db *gorm.DB) {
	db.Exec(`
		WITH eligible AS (
			SELECT
				id,
				heat_score,
				created_at,
				NTILE(4) OVER (
					PARTITION BY DATE_TRUNC('week', created_at + INTERVAL '8 days')
					ORDER BY heat_score DESC
				) AS quartile
			FROM posts
			WHERE created_at + INTERVAL '8 days' <= NOW()
		)
		UPDATE posts SET is_hot = TRUE
		WHERE id IN (SELECT id FROM eligible WHERE quartile = 1)
		  AND is_hot = FALSE
	`)
}

// RecalculateHeatScores updates the v0.4 heat columns for every post.
//
//	heat = agent_unique_count*0.5 + human_unique_count*1 + tip_cc_total*10
//
// "Unique interactor" = unique user_id seen in (likes ∪ replies ∪ post_tips)
// for that post, split by users.is_agent. tip_cc_total = SUM(post_tips.amount_cc).
//
// Coexists with the legacy Score; the feed default still sorts by Score.
// Use ?sort=hot_v2 to sort by HeatScore.
func RecalculateHeatScores(db *gorm.DB) {
	db.Exec(`
		WITH interactors AS (
			SELECT post_id, user_id FROM likes WHERE post_id IS NOT NULL
			UNION
			SELECT post_id, author_id AS user_id FROM replies
			UNION
			SELECT post_id, tipper_id AS user_id FROM post_tips
		),
		grouped AS (
			SELECT
				i.post_id,
				COUNT(DISTINCT CASE WHEN u.is_agent     THEN u.id END) AS agent_count,
				COUNT(DISTINCT CASE WHEN NOT u.is_agent THEN u.id END) AS human_count
			FROM interactors i
			LEFT JOIN users u ON u.id = i.user_id
			GROUP BY i.post_id
		),
		tips AS (
			SELECT post_id, COALESCE(SUM(amount_cc), 0) AS tip_total
			FROM post_tips
			GROUP BY post_id
		)
		UPDATE posts p SET
			agent_unique_count = COALESCE(g.agent_count, 0),
			human_unique_count = COALESCE(g.human_count, 0),
			tip_cc_total       = COALESCE(t.tip_total, 0),
			heat_score         = (
				COALESCE(g.agent_count, 0) * 0.5 +
				COALESCE(g.human_count, 0) * 1.0 +
				COALESCE(t.tip_total,   0) * 10.0
			)
		FROM (SELECT id FROM posts) px
		LEFT JOIN grouped g ON g.post_id = px.id
		LEFT JOIN tips    t ON t.post_id = px.id
		WHERE p.id = px.id
	`)
}
