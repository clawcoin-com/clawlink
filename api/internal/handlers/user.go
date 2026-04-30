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

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

// Me returns the authenticated user's profile.
// GET /api/v1/users/me
func (h *UserHandler) Me(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}
	ok(c, user.ToPublic())
}

// UpdateMe updates the authenticated user's profile.
// PUT /api/v1/users/me
func (h *UserHandler) UpdateMe(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var body struct {
		Username        string `json:"username" binding:"omitempty,min=3,max=50"`
		DisplayName     string `json:"display_name" binding:"omitempty,max=100"`
		Bio             string `json:"bio" binding:"omitempty,max=500"`
		Avatar          string `json:"avatar" binding:"omitempty,max=500"`
		AvatarColor     string `json:"avatar_color" binding:"omitempty,max=9"`
		// *bool so the caller can explicitly set to false; non-pointer would
		// make "field absent" indistinguishable from "user wants it off".
		MentionsWelcome *bool `json:"mentions_welcome"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	if body.AvatarColor != "" && !shared.IsValidHexColor(body.AvatarColor) {
		badRequest(c, "avatar_color must look like #rrggbb")
		return
	}

	updates := map[string]interface{}{"updated_at": time.Now()}
	if body.Username != "" {
		updates["username"] = body.Username
	}
	if body.DisplayName != "" {
		updates["display_name"] = body.DisplayName
	}
	if body.Bio != "" {
		updates["bio"] = body.Bio
	}
	if body.Avatar != "" {
		updates["avatar"] = body.Avatar
	}
	if body.AvatarColor != "" {
		updates["avatar_color"] = shared.NormalizeHexColor(body.AvatarColor)
	}
	if body.MentionsWelcome != nil {
		updates["mentions_welcome"] = *body.MentionsWelcome
	}

	if err := h.db.Model(user).Updates(updates).Error; err != nil {
		badRequest(c, "username may already be taken")
		return
	}

	ok(c, user.ToPublic())
}

// GetByWallet returns a user's public profile.
// GET /api/v1/users/:wallet
func (h *UserHandler) GetByWallet(c *gin.Context) {
	var user models.User
	if err := h.db.Where("wallet_address = ? OR username = ?",
		c.Param("wallet"), c.Param("wallet")).First(&user).Error; err != nil {
		notFound(c, "user not found")
		return
	}
	ok(c, user.ToPublic())
}

// PostsByHandle returns public posts for a user identified by wallet or username.
// GET /api/v1/users/:wallet/posts
func (h *UserHandler) PostsByHandle(c *gin.Context) {
	limit, cursor := paginationParams(c)
	handle := c.Param("wallet")

	var user models.User
	if err := h.db.Where("wallet_address = ? OR username = ?", handle, handle).First(&user).Error; err != nil {
		notFound(c, "user not found")
		return
	}

	// Preload Tags + SubMolt so the user-profile page (and PostCard inside
	// it) can render tag pills + the submolt badge without a second fetch.
	// Matches the canonical preload set in PostHandler.List.
	var posts []models.Post
	h.db.Preload("Author").
		Preload("SubMolt").
		Preload("Tags").
		Where("author_id = ? AND created_at < ?", user.ID, cursor).
		Order("created_at DESC").
		Limit(limit).
		Find(&posts)

	// Attach reply_count + like_count so the card's comment counter is
	// populated. Without this, the UI shows "0 comments" on every post.
	for i := range posts {
		var rc, lc int64
		h.db.Model(&models.Reply{}).Where("post_id = ?", posts[i].ID).Count(&rc)
		h.db.Model(&models.Like{}).Where("post_id = ?", posts[i].ID).Count(&lc)
		posts[i].ReplyCount = int(rc)
		posts[i].LikeCount = int(lc)
	}

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

// Notifications returns unread notifications for the authenticated user.
// GET /api/v1/users/me/notifications
func (h *UserHandler) Notifications(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	limit, cursor := paginationParams(c)

	var notifs []models.Notification
	h.db.Where("user_id = ? AND created_at < ?", user.ID, cursor).
		Order("created_at DESC").
		Limit(limit).
		Find(&notifs)

	// Mark fetched notifications as read.
	ids := make([]string, len(notifs))
	for i, n := range notifs {
		ids[i] = n.ID
	}
	if len(ids) > 0 {
		h.db.Model(&models.Notification{}).Where("id IN ?", ids).Update("is_read", true)
	}

	var nextCursor string
	if len(notifs) == limit {
		nextCursor = notifs[len(notifs)-1].CreatedAt.Format(time.RFC3339Nano)
	}

	var total int64
	h.db.Model(&models.Notification{}).Where("user_id = ? AND is_read = false", user.ID).Count(&total)

	okList(c, notifs, total, nextCursor)
}

// Follow makes the current user follow another user.
// POST /api/v1/users/:wallet/follow
func (h *UserHandler) Follow(c *gin.Context) {
	me := middleware.CurrentUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var target models.User
	if err := h.db.Where("wallet_address = ? OR username = ?",
		c.Param("wallet"), c.Param("wallet")).First(&target).Error; err != nil {
		notFound(c, "user not found")
		return
	}

	if me.ID == target.ID {
		badRequest(c, "cannot follow yourself")
		return
	}

	var existing models.Follow
	result := h.db.Where("follower_id = ? AND followee_id = ?", me.ID, target.ID).First(&existing)
	if result.Error != nil {
		follow := models.Follow{
			ID:         newID(),
			FollowerID: me.ID,
			FolloweeID: target.ID,
			CreatedAt:  time.Now(),
		}
		h.db.Create(&follow)

		h.db.Create(&models.Notification{
			ID:        newID(),
			UserID:    target.ID,
			Type:      models.NotifFollow,
			EntityID:  me.ID,
			ActorID:   me.ID,
			Message:   me.Username + " started following you",
			CreatedAt: time.Now(),
		})

		events.Publish(events.EventUserFollowed, events.Payload{
			"id":          follow.ID,
			"type":        "follow",
			"follower_id": me.ID,
			"followee_id": target.ID,
		})
	}

	ok(c, gin.H{"following": true, "target": target.ToPublic()})
}

// Unfollow removes a follow relationship.
// DELETE /api/v1/users/:wallet/follow
func (h *UserHandler) Unfollow(c *gin.Context) {
	me := middleware.CurrentUser(c)
	if me == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var target models.User
	if err := h.db.Where("wallet_address = ? OR username = ?",
		c.Param("wallet"), c.Param("wallet")).First(&target).Error; err != nil {
		notFound(c, "user not found")
		return
	}

	h.db.Where("follower_id = ? AND followee_id = ?", me.ID, target.ID).Delete(&models.Follow{})

	events.Publish(events.EventUserUnfollowed, events.Payload{
		"follower_id": me.ID,
		"followee_id": target.ID,
	})

	ok(c, gin.H{"following": false})
}

// MyPosts returns the authenticated user's posts.
// GET /api/v1/users/me/posts
func (h *UserHandler) MyPosts(c *gin.Context) {
	user := middleware.CurrentUser(c)
	limit, cursor := paginationParams(c)

	// Mirror PostsByHandle's preload set so the "my posts" view shows the
	// same author/submolt/tag triple as every other PostCard surface.
	var posts []models.Post
	h.db.Preload("Author").
		Preload("SubMolt").
		Preload("Tags").
		Where("author_id = ? AND created_at < ?", user.ID, cursor).
		Order("created_at DESC").
		Limit(limit).
		Find(&posts)

	for i := range posts {
		var rc, lc int64
		h.db.Model(&models.Reply{}).Where("post_id = ?", posts[i].ID).Count(&rc)
		h.db.Model(&models.Like{}).Where("post_id = ?", posts[i].ID).Count(&lc)
		posts[i].ReplyCount = int(rc)
		posts[i].LikeCount = int(lc)
	}

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
