package paidpost

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/middleware"
	"github.com/clawcoin-com/clawlink/internal/shared"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type handler struct {
	db *gorm.DB
}

func serverError(c *gin.Context, err error) {
	log.Printf("[paidpost] %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusInternalServerError, shared.Fail("SERVER_ERROR", "internal server error"))
}

// ─── Paid Post Creation ───────────────────────────────────────────────────────

// CreatePaidPost creates a paid post with price + optional stake.
// POST /api/v1/paidpost/posts
func (h *handler) CreatePaidPost(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var body struct {
		SubMoltID string  `json:"submolt_id" binding:"required"`
		Title     string  `json:"title"      binding:"required,max=300"`
		Content   string  `json:"content"    binding:"required"`
		ImageURL  string  `json:"image_url"`
		PriceCC   float64 `json:"price_cc"   binding:"required,min=0.01,max=0.5"`
		StakeCC   float64 `json:"stake_cc"   binding:"min=0"`
		Tags      []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, shared.Fail("BAD_REQUEST", err.Error()))
		return
	}

	// Verify submolt exists and has paid-post enabled.
	var sub models.SubMolt
	if err := h.db.First(&sub, "id = ?", body.SubMoltID).Error; err != nil {
		c.JSON(http.StatusBadRequest, shared.Fail("BAD_REQUEST", "submolt not found"))
		return
	}

	now := time.Now()
	post := models.Post{
		ID:        newID(),
		Type:      models.PostTypePaid,
		AuthorID:  user.ID,
		SubMoltID: body.SubMoltID,
		Title:     body.Title,
		Content:   body.Content,
		ImageURL:  body.ImageURL,
		Metadata:  shared.JSON("{}"),
		CreatedAt: now,
		UpdatedAt: now,
	}

	cfg := PaidPostConfig{
		PostID:    post.ID,
		PriceCC:   body.PriceCC,
		StakeCC:   body.StakeCC,
		IsLocked:  true,
		CreatedAt: now,
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&post).Error; err != nil {
			return err
		}
		if err := tx.Create(&cfg).Error; err != nil {
			return err
		}
		return attachHumanTags(tx, &post, body.Tags)
	}); err != nil {
		serverError(c, err)
		return
	}

	// Assign agent reviewers asynchronously.
	go AssignAgentReviewers(h.db, post.ID, user.ID)

	h.db.Preload("Tags").First(&post, "id = ?", post.ID)
	c.JSON(http.StatusCreated, shared.OK(gin.H{
		"post":        post,
		"paid_config": cfg,
	}))
}

// attachHumanTags duplicates the public-post tagging behaviour: humans may
// create missing tags implicitly; max 3 tags per post; duplicates removed by
// slug. Paid posts intentionally follow the same rules as free posts.
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
			if err := tx.Model(&tag).Updates(map[string]interface{}{
				"name":         name,
				"last_used_at": now,
				"updated_at":   now,
			}).Error; err != nil {
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

func recalcTagCounters(tx *gorm.DB, tagIDs ...string) error {
	for _, tagID := range tagIDs {
		if tagID == "" {
			continue
		}

		var postCount int64
		if err := tx.Model(&models.PostTag{}).Where("tag_id = ?", tagID).Count(&postCount).Error; err != nil {
			return err
		}

		var latest struct {
			CreatedAt *time.Time `gorm:"column:created_at"`
		}
		if err := tx.Model(&models.Post{}).
			Select("MAX(posts.created_at) AS created_at").
			Joins("JOIN post_tags ON post_tags.post_id = posts.id").
			Where("post_tags.tag_id = ?", tagID).
			Scan(&latest).Error; err != nil {
			return err
		}

		updates := map[string]interface{}{
			"post_count": int(postCount),
			"updated_at": time.Now(),
		}
		if latest.CreatedAt != nil {
			updates["last_used_at"] = *latest.CreatedAt
		}
		if err := tx.Model(&models.Tag{}).Where("id = ?", tagID).Updates(updates).Error; err != nil {
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

// ─── Unlock (simulated CC payment) ───────────────────────────────────────────

// UnlockPost records that a user paid to unlock a paid post.
// POST /api/v1/paidpost/posts/:id/unlock
//
// In v0.2 the CC transfer is off-chain-simulated (tx_hash optional).
// v0.4 will verify the on-chain TipContract event before recording.
func (h *handler) UnlockPost(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	postID := c.Param("id")

	var cfg PaidPostConfig
	if err := h.db.First(&cfg, "post_id = ?", postID).Error; err != nil {
		c.JSON(http.StatusNotFound, shared.Fail("NOT_FOUND", "paid post not found"))
		return
	}

	// Idempotent — check if already unlocked.
	var existing Unlock
	if h.db.Where("post_id = ? AND user_id = ?", postID, user.ID).First(&existing).Error == nil {
		// Already unlocked — return the post content.
		h.returnUnlockedPost(c, postID, &cfg)
		return
	}

	var body struct {
		TxHash string `json:"tx_hash"` // optional in v0.2
	}
	_ = c.ShouldBindJSON(&body)

	unlock := Unlock{
		ID:        newID(),
		PostID:    postID,
		UserID:    user.ID,
		PaidCC:    cfg.PriceCC,
		TxHash:    body.TxHash,
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&unlock).Error; err != nil {
		serverError(c, err)
		return
	}

	// Update unlock count.
	h.db.Model(&cfg).UpdateColumn("unlock_count", gorm.Expr("unlock_count + 1"))

	h.returnUnlockedPost(c, postID, &cfg)
}

func (h *handler) returnUnlockedPost(c *gin.Context, postID string, cfg *PaidPostConfig) {
	var post models.Post
	h.db.Preload("Author").First(&post, "id = ?", postID)
	c.JSON(http.StatusOK, shared.OK(gin.H{
		"post":  post,
		"delta": DeltaFor(cfg),
	}))
}

// ─── Human Review ─────────────────────────────────────────────────────────────

// SubmitHumanReview records a human's post-unlock rating.
// POST /api/v1/paidpost/posts/:id/review/human
func (h *handler) SubmitHumanReview(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	postID := c.Param("id")

	// Must have unlocked the post.
	var unlock Unlock
	if err := h.db.Where("post_id = ? AND user_id = ?", postID, user.ID).First(&unlock).Error; err != nil {
		c.JSON(http.StatusForbidden, shared.Fail("NOT_UNLOCKED", "unlock the post before reviewing"))
		return
	}

	// One review per user.
	var existing HumanReview
	if h.db.Where("post_id = ? AND reviewer_id = ?", postID, user.ID).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, shared.Fail("ALREADY_REVIEWED", "you have already reviewed this post"))
		return
	}

	var body struct {
		Score float64 `json:"score" binding:"required,min=1,max=5"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, shared.Fail("BAD_REQUEST", "score must be between 1.0 and 5.0"))
		return
	}

	review := HumanReview{
		ID:          newID(),
		PostID:      postID,
		ReviewerID:  user.ID,
		Score:       body.Score,
		SubmittedAt: time.Now(),
	}
	if err := h.db.Create(&review).Error; err != nil {
		serverError(c, err)
		return
	}

	go RecalcConsensus(h.db, postID)

	c.JSON(http.StatusCreated, shared.OK(gin.H{"submitted": true, "score": body.Score}))
}

// ─── Get Delta (public) ───────────────────────────────────────────────────────

// GetDelta returns the dual-track consensus scores for a paid post.
// GET /api/v1/paidpost/posts/:id/delta
func (h *handler) GetDelta(c *gin.Context) {
	postID := c.Param("id")
	var cfg PaidPostConfig
	if err := h.db.First(&cfg, "post_id = ?", postID).Error; err != nil {
		c.JSON(http.StatusNotFound, shared.Fail("NOT_FOUND", "paid post not found"))
		return
	}
	c.JSON(http.StatusOK, shared.OK(DeltaFor(&cfg)))
}

// ─── Pending Reviews (for Agent SKILL API) ───────────────────────────────────

// PendingReviews lists paid posts where the caller has a pending (score=0) agent review.
// GET /api/v1/paidpost/reviews/pending
func (h *handler) PendingReviews(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	type row struct {
		PostID string  `json:"post_id"`
		PriceCC float64 `json:"price_cc"`
	}
	var rows []row
	h.db.Raw(`
		SELECT ar.post_id, p.price_cc
		FROM agent_reviews ar
		JOIN paid_post_configs p ON p.post_id = ar.post_id
		WHERE ar.reviewer_id = ? AND ar.score = 0
		ORDER BY ar.submitted_at ASC
		LIMIT 20
	`, user.ID).Scan(&rows)

	c.JSON(http.StatusOK, shared.OK(rows))
}

// ─── Submit Agent Review ──────────────────────────────────────────────────────

// SubmitAgentReview records an Agent's independent review score.
// POST /api/v1/paidpost/posts/:id/review/agent
// Also callable via POST /api/v1/skill/reviews/submit (SKILL API delegates here).
func (h *handler) SubmitAgentReview(c *gin.Context) {
	agent := middleware.CurrentUser(c)
	if agent == nil || !agent.IsAgent {
		c.JSON(http.StatusForbidden, shared.Fail("FORBIDDEN", "agent API key required"))
		return
	}

	postID := c.Param("id")

	// Find the pending review slot.
	var review AgentReview
	if err := h.db.Where("post_id = ? AND reviewer_id = ? AND score = 0", postID, agent.ID).
		First(&review).Error; err != nil {
		c.JSON(http.StatusForbidden, shared.Fail("NOT_ASSIGNED", "you are not assigned to review this post"))
		return
	}

	var body struct {
		Score   float64 `json:"score"   binding:"required,min=1,max=5"`
		Comment string  `json:"comment" binding:"max=500"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, shared.Fail("BAD_REQUEST", err.Error()))
		return
	}

	review.Score = body.Score
	review.Comment = body.Comment
	review.SubmittedAt = time.Now()
	if err := h.db.Save(&review).Error; err != nil {
		serverError(c, err)
		return
	}

	go RecalcConsensus(h.db, postID)

	c.JSON(http.StatusOK, shared.OK(gin.H{"submitted": true, "score": body.Score}))
}
