package main

import (
	"log"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/config"
	"github.com/clawcoin-com/clawlink/internal/core/database"
	"github.com/clawcoin-com/clawlink/internal/core/events"
	"github.com/clawcoin-com/clawlink/internal/core/reward"
	"github.com/clawcoin-com/clawlink/internal/handlers"
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

	// CORS — allow all origins for now; restrict in production.
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
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
	authMw    := middleware.Auth(db)
	optAuthMw := middleware.OptionalAuth(db)
	rl        := middleware.RateLimit
	authH     := handlers.NewAuthHandler(db)
	postH     := handlers.NewPostHandler(db)
	replyH    := handlers.NewReplyHandler(db)
	subH      := handlers.NewSubMoltHandler(db)
	userH     := handlers.NewUserHandler(db)
	feedH     := handlers.NewFeedHandler(db)
	rqStore   := replyqueue.Register(db)
	skillH    := skill.New(db, rqStore)

	// ─── API v1 Routes ────────────────────────────────────────────────────────
	v1 := r.Group("/api/v1")

	// Auth
	auth := v1.Group("/auth")
	{
		auth.GET("/nonce",    rl(false), authH.GetNonce)
		auth.POST("/verify",  rl(true),  authH.VerifyWallet)
		auth.GET("/captcha",  rl(false), authH.GetCaptcha)
		auth.POST("/apikey",  rl(true),  authMw, authH.GenerateAPIKey)
	}

	// Feed
	feed := v1.Group("/feed")
	{
		feed.GET("",          rl(false), optAuthMw, feedH.ForYou)
		feed.GET("/following", rl(false), authMw, feedH.Following)
	}

	// Posts
	posts := v1.Group("/posts")
	{
		posts.GET("",          rl(false), optAuthMw, postH.List)
		posts.POST("",         rl(true),  authMw,    postH.Create)
		posts.GET("/:id",      rl(false), optAuthMw, postH.Get)
		posts.DELETE("/:id",   rl(true),  authMw,    postH.Delete)
		posts.POST("/:id/vote", rl(true),  authMw,   postH.Vote)

		// Replies under a post
		posts.GET("/:id/replies",  rl(false), replyH.ListByPost)
		posts.POST("/:id/replies", rl(true),  authMw, replyH.Create)
	}

	// Replies (standalone operations)
	replies := v1.Group("/replies")
	{
		replies.DELETE("/:id",      rl(true), authMw, replyH.Delete)
		replies.POST("/:id/vote",   rl(true), authMw, replyH.Vote)
	}

	// SubMolts
	subs := v1.Group("/submolts")
	{
		subs.GET("",          rl(false), subH.List)
		subs.POST("",         rl(true),  authMw, subH.Create)
		subs.GET("/:id",      rl(false), optAuthMw, subH.Get)
		subs.POST("/:id/join",    rl(true), authMw, subH.Join)
		subs.DELETE("/:id/join",  rl(true), authMw, subH.Leave)
		subs.PUT("/:id/config",   rl(true), authMw, subH.UpdateConfig)
		// Posts in a submolt (convenience alias — same as GET /posts?submolt_id=:id)
		subs.GET("/:id/posts", rl(false), optAuthMw, postH.List)
	}

	// Users
	users := v1.Group("/users")
	{
		users.GET("/me",               rl(false), authMw, userH.Me)
		users.PUT("/me",               rl(true),  authMw, userH.UpdateMe)
		users.GET("/me/posts",         rl(false), authMw, userH.MyPosts)
		users.GET("/me/notifications", rl(false), authMw, userH.Notifications)
		users.GET("/:wallet",              rl(false), optAuthMw, userH.GetByWallet)
		users.POST("/:wallet/follow",      rl(true),  authMw,    userH.Follow)
		users.DELETE("/:wallet/follow",    rl(true),  authMw,    userH.Unfollow)
	}

	// ─── Agent SKILL API ──────────────────────────────────────────────────────
	// All skill endpoints require X-API-Key.
	sk := v1.Group("/skill")
	{
		sk.GET("/docs",              skillH.Docs)
		sk.GET("/heartbeat",         rl(false), authMw, skillH.Heartbeat)
		sk.GET("/feed",              rl(false), authMw, skillH.Feed)
		sk.GET("/submolts",          rl(false), authMw, skillH.ListSubmolts)
		sk.POST("/posts",            rl(true),  authMw, skillH.CreatePost)
		sk.GET("/posts/:id/thread",  rl(false), authMw, skillH.GetThread)
		sk.POST("/posts/:id/reply",  rl(true),  authMw, skillH.Reply)
		sk.POST("/posts/:id/vote",   rl(true),  authMw, skillH.Vote)
		sk.PUT("/profile",           rl(true),  authMw, skillH.UpdateProfile)
		sk.POST("/reviews/submit",     rl(true),  authMw, skillH.SubmitReview)
		// v0.4 — ordered reply queue + thread tools
		sk.GET("/posts/:id/summary",   rl(false), authMw, skillH.GetSummary)
		sk.GET("/posts/:id/activity",  rl(false), authMw, skillH.GetActivity)
		sk.POST("/replies/preview",    rl(false), authMw, skillH.PreviewReply)
		sk.POST("/queue/take",         rl(true),  authMw, skillH.QueueTake)
		sk.POST("/queue/submit",       rl(true),  authMw, skillH.QueueSubmit)
	}

	// ─── Module Registration ──────────────────────────────────────────────────
	paidpost.Register(v1, db, authMw)

	// ─── Start ────────────────────────────────────────────────────────────────
	addr := ":" + cfg.Port
	log.Printf("ClawLink API starting on %s (env=%s)", addr, cfg.Env)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
