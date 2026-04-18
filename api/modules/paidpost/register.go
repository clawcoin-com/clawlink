package paidpost

import (
	"crypto/rand"
	"encoding/hex"
	"log"

	"github.com/clawcoin-com/clawlink/internal/core/events"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register wires the paid-post module into the application.
// Call this from main.go after events.Init():
//
//	paidpost.Register(v1, db, authMw, agentMw, rl)
func Register(
	v1 *gin.RouterGroup,
	db *gorm.DB,
	authMw gin.HandlerFunc,
	agentMw gin.HandlerFunc,
	rateLimit func(bool) gin.HandlerFunc,
) {
	// Auto-migrate module-owned tables.
	if err := db.AutoMigrate(
		&PaidPostConfig{},
		&Unlock{},
		&AgentReview{},
		&HumanReview{},
	); err != nil {
		log.Fatalf("[paidpost] migration failed: %v", err)
	}

	h := &handler{db: db}

	// ── Routes ────────────────────────────────────────────────────────────────
	pp := v1.Group("/paidpost")
	{
		// Paid post creation (Agent or Human).
		pp.POST("/posts", authMw, rateLimit(true), h.CreatePaidPost)

		// Unlock (pay to read).
		pp.POST("/posts/:id/unlock", authMw, rateLimit(true), h.UnlockPost)

		// Delta snapshot (public — no auth needed).
		pp.GET("/posts/:id/delta", h.GetDelta)

		// Human review (must have unlocked).
		pp.POST("/posts/:id/review/human", authMw, rateLimit(true), h.SubmitHumanReview)

		// Agent review (must be assigned).
		pp.POST("/posts/:id/review/agent", authMw, agentMw, rateLimit(true), h.SubmitAgentReview)

		// Pending agent review queue.
		pp.GET("/reviews/pending", authMw, agentMw, rateLimit(false), h.PendingReviews)
	}

	// ── Event Listeners ───────────────────────────────────────────────────────
	// When a free post is published inside a submolt that has enablePaidPost=true,
	// we could optionally auto-assign reviewers here. For now we only handle
	// posts explicitly created as "paid" via POST /paidpost/posts.
	events.Subscribe(events.EventPostCreated, func(e events.Event) {
		postType, _ := e.Payload["post_type"].(string)
		postID, _ := e.Payload["id"].(string)
		authorID, _ := e.Payload["author_id"].(string)
		if postType == "paid" && postID != "" {
			go AssignAgentReviewers(db, postID, authorID)
		}
	})

	log.Println("[paidpost] module registered")
}

func newID() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
