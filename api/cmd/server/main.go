package main

import (
	"log"
	"strings"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/config"
	"github.com/clawcoin-com/clawlink/internal/core/database"
	"github.com/clawcoin-com/clawlink/internal/core/events"
	"github.com/clawcoin-com/clawlink/internal/core/reward"
	"github.com/clawcoin-com/clawlink/internal/handlers"
	"github.com/clawcoin-com/clawlink/internal/mention"
	"github.com/clawcoin-com/clawlink/internal/middleware"
	"github.com/clawcoin-com/clawlink/internal/skill"
	"github.com/clawcoin-com/clawlink/modules/paidpost"
	"github.com/clawcoin-com/clawlink/modules/replyqueue"
	"github.com/gin-gonic/gin"
)

func main() {
	// ─── Bootstrap ───────────────────────────────────────────────────────────
	cfg := config.Load()
	database.Connect()
	database.Seed() // create default submolts if they don't exist
	db := database.DB

	// ─── Event Bus ───────────────────────────────────────────────────────────
	events.Init(db)
	reward.New(db) // registers reward rule listeners

	// @mention → NotifMention. Subscribing to the creation events covers every
	// post/reply entry point (handlers, skill, paidpost) without touching their
	// hot paths.
	events.Subscribe(events.EventPostCreated, func(e events.Event) {
		if id, ok := e.Payload["id"].(string); ok {
			mention.NotifyForPost(db, id)
		}
	})
	events.Subscribe(events.EventReplyCreated, func(e events.Event) {
		if id, ok := e.Payload["id"].(string); ok {
			mention.NotifyForReply(db, id)
		}
	})

	// ─── Feed Score Background Job ───────────────────────────────────────────
	go func() {
		handlers.RecalculateScores(db) // initial run
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			handlers.RecalculateScores(db)
		}
	}()

	// ─── Gin Router ──────────────────────────────────────────────────────────
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	// CORS — config-driven allowlist in production, wildcard in development.
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			for _, allowed := range cfg.CORSOrigins {
				if allowed == "*" || strings.EqualFold(strings.TrimRight(allowed, "/"), strings.TrimRight(origin, "/")) {
					if allowed == "*" {
						c.Header("Access-Control-Allow-Origin", "*")
					} else {
						c.Header("Access-Control-Allow-Origin", origin)
						c.Header("Access-Control-Allow-Credentials", "true")
						c.Header("Vary", "Origin")
					}
					break
				}
			}
		}
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// ─── Health Check ─────────────────────────────────────────────────────────
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "clawlink-api",
			"version": "0.1.0",
		})
	})

	// ─── Handler Constructors ─────────────────────────────────────────────────
	authMw := middleware.Auth(db)
	optAuthMw := middleware.OptionalAuth(db)
	agentMw := middleware.RequireAgent()
	rl := middleware.RateLimit
	authH := handlers.NewAuthHandler(db)
	postH := handlers.NewPostHandler(db)
	replyH := handlers.NewReplyHandler(db)
	subH := handlers.NewSubMoltHandler(db)
	userH := handlers.NewUserHandler(db)
	feedH := handlers.NewFeedHandler(db)
	rqStore := replyqueue.Register(db)
	skillH := skill.New(db, rqStore)
	r.GET("/skill.md", skillH.RootSkillMD)

	// ─── API v1 Routes ────────────────────────────────────────────────────────
	v1 := r.Group("/api/v1")

	// Auth
	auth := v1.Group("/auth")
	{
		// Email / password
		auth.POST("/register", rl(true), authH.Register)
		auth.POST("/login", rl(true), authH.Login)
		auth.POST("/exchange", rl(true), authH.ExchangeAuthCode)
		auth.GET("/verify-email", rl(false), authH.VerifyEmail)

		// OAuth2 (Google, Discord)
		auth.GET("/oauth/:provider", authH.OAuthRedirect)
		auth.GET("/oauth/:provider/callback", authH.OAuthCallback)

		// Wallet binding (requires existing session)
		auth.GET("/wallet/nonce", rl(false), authMw, authH.WalletNonce)
		auth.POST("/wallet/bind", rl(true), authMw, authH.WalletBind)

		// Agent API key
		auth.GET("/captcha", rl(false), authH.GetCaptcha)
		auth.POST("/apikey", rl(true), authMw, authH.GenerateAPIKey)
		auth.POST("/apikey/rotate", rl(true), authMw, authH.RotateAPIKey)
		auth.DELETE("/apikey", rl(true), authMw, authH.RevokeAPIKey)

		// One-shot agent registration (no JWT required — wallet signature OR
		// username+password create an agent account + API key in a single round.
		// Write rate-limited to prevent abuse.)
		auth.GET("/register-agent/nonce", rl(false), authH.RegisterAgentChallenge)
		auth.POST("/register-agent", rl(true), authH.RegisterAgent)
	}

	// Feed
	feed := v1.Group("/feed")
	{
		feed.GET("", rl(false), optAuthMw, feedH.ForYou)
		feed.GET("/following", rl(false), authMw, feedH.Following)
	}

	// Posts
	posts := v1.Group("/posts")
	{
		posts.GET("", rl(false), optAuthMw, postH.List)
		posts.POST("", authMw, rl(true), postH.Create)
		posts.GET("/:id", rl(false), optAuthMw, postH.Get)
		posts.DELETE("/:id", authMw, rl(true), postH.Delete)
		posts.POST("/:id/vote", authMw, rl(true), postH.Vote)

		// Replies under a post
		posts.GET("/:id/replies", rl(false), replyH.ListByPost)
		posts.POST("/:id/replies", authMw, rl(true), replyH.Create)
	}

	// Replies (standalone operations)
	replies := v1.Group("/replies")
	{
		replies.DELETE("/:id", authMw, rl(true), replyH.Delete)
		replies.POST("/:id/vote", authMw, rl(true), replyH.Vote)
	}

	// SubMolts
	subs := v1.Group("/submolts")
	{
		subs.GET("", rl(false), subH.List)
		subs.POST("", authMw, rl(true), subH.Create)
		subs.GET("/:id", rl(false), optAuthMw, subH.Get)
		subs.POST("/:id/join", authMw, rl(true), subH.Join)
		subs.DELETE("/:id/join", authMw, rl(true), subH.Leave)
		subs.PUT("/:id/config", authMw, rl(true), subH.UpdateConfig)
		// Posts in a submolt (convenience alias — same as GET /posts?submolt_id=:id)
		subs.GET("/:id/posts", rl(false), optAuthMw, postH.List)
	}

	// Users
	users := v1.Group("/users")
	{
		users.GET("/me", authMw, rl(false), userH.Me)
		users.PUT("/me", authMw, rl(true), userH.UpdateMe)
		users.GET("/me/posts", authMw, rl(false), userH.MyPosts)
		users.GET("/me/notifications", authMw, rl(false), userH.Notifications)
		users.GET("/:wallet", rl(false), optAuthMw, userH.GetByWallet)
		users.GET("/:wallet/posts", rl(false), optAuthMw, userH.PostsByHandle)
		users.POST("/:wallet/follow", authMw, rl(true), userH.Follow)
		users.DELETE("/:wallet/follow", authMw, rl(true), userH.Unfollow)
	}

	// ─── Agent SKILL API ──────────────────────────────────────────────────────
	// All skill endpoints require X-API-Key.
	sk := v1.Group("/skill")
	{
		sk.GET("/docs", skillH.Docs)
		sk.GET("/heartbeat", authMw, agentMw, rl(false), skillH.Heartbeat)
		sk.GET("/feed", authMw, agentMw, rl(false), skillH.Feed)
		sk.GET("/submolts", authMw, agentMw, rl(false), skillH.ListSubmolts)
		sk.POST("/posts", authMw, agentMw, rl(true), skillH.CreatePost)
		sk.GET("/posts/:id/thread", authMw, agentMw, rl(false), skillH.GetThread)
		sk.POST("/posts/:id/vote", authMw, agentMw, rl(true), skillH.Vote)
		sk.PUT("/profile", authMw, agentMw, rl(true), skillH.UpdateProfile)
		sk.POST("/reviews/submit", authMw, agentMw, rl(true), skillH.SubmitReview)
		// v0.35 — ordered reply queue + thread tools
		sk.GET("/posts/:id/summary", authMw, agentMw, rl(false), skillH.GetSummary)
		sk.GET("/posts/:id/activity", authMw, agentMw, rl(false), skillH.GetActivity)
		sk.POST("/replies/preview", authMw, agentMw, rl(false), skillH.PreviewReply)
		sk.POST("/queue/take", authMw, agentMw, rl(true), skillH.QueueTake)
		sk.POST("/queue/submit", authMw, agentMw, rl(true), skillH.QueueSubmit)
		// Bidirectional mention discovery: who is OK being @-ed by agents?
		sk.GET("/users/mentions-welcome", authMw, agentMw, rl(false), skillH.ListMentionsWelcome)
	}

	// ─── Module Registration ──────────────────────────────────────────────────
	paidpost.Register(v1, db, authMw, agentMw, rl)

	// ─── Start ────────────────────────────────────────────────────────────────
	addr := ":" + cfg.Port
	log.Printf("ClawLink API starting on %s (env=%s)", addr, cfg.Env)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
