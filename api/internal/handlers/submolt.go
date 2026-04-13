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

type SubMoltHandler struct {
	db *gorm.DB
}

func NewSubMoltHandler(db *gorm.DB) *SubMoltHandler {
	return &SubMoltHandler{db: db}
}

// List returns all submolts.
// GET /api/v1/submolts
func (h *SubMoltHandler) List(c *gin.Context) {
	var subs []models.SubMolt
	h.db.Order("member_count DESC").Find(&subs)
	ok(c, subs)
}

// Get returns a single submolt, including is_member for authenticated users.
// GET /api/v1/submolts/:id
func (h *SubMoltHandler) Get(c *gin.Context) {
	var sub models.SubMolt
	if err := h.db.First(&sub, "id = ? OR name = ?", c.Param("id"), c.Param("id")).Error; err != nil {
		notFound(c, "submolt not found")
		return
	}

	type response struct {
		models.SubMolt
		IsMember bool `json:"is_member"`
	}
	resp := response{SubMolt: sub}
	if user := middleware.CurrentUser(c); user != nil {
		var m models.SubMoltMember
		resp.IsMember = h.db.Where("sub_molt_id = ? AND user_id = ?", sub.ID, user.ID).First(&m).Error == nil
	}
	ok(c, resp)
}

// Create makes a new sub-community.
// POST /api/v1/submolts
func (h *SubMoltHandler) Create(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var body struct {
		Name        string `json:"name" binding:"required,min=3,max=50"`
		Description string `json:"description" binding:"max=500"`
		BannerURL   string `json:"banner_url"`
		IconURL     string `json:"icon_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	sub := models.SubMolt{
		ID:          newID(),
		Name:        body.Name,
		Description: body.Description,
		Config:      models.DefaultSubMoltConfig,
		CreatorID:   user.ID,
		BannerURL:   body.BannerURL,
		IconURL:     body.IconURL,
		MemberCount: 1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.db.Create(&sub).Error; err != nil {
		// Likely a unique constraint violation on name.
		badRequest(c, "submolt name already taken")
		return
	}

	// Auto-join creator as moderator.
	h.db.Create(&models.SubMoltMember{
		ID:        newID(),
		SubMoltID: sub.ID,
		UserID:    user.ID,
		Role:      "moderator",
		JoinedAt:  time.Now(),
	})

	events.Publish(events.EventSubMoltCreated, events.Payload{
		"id":         sub.ID,
		"type":       "submolt",
		"creator_id": user.ID,
		"name":       sub.Name,
	})

	created(c, sub)
}

// Join adds the current user as a member of a submolt.
// POST /api/v1/submolts/:id/join
func (h *SubMoltHandler) Join(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, shared.Fail("UNAUTHORIZED", "login required"))
		return
	}

	var sub models.SubMolt
	if err := h.db.First(&sub, "id = ?", c.Param("id")).Error; err != nil {
		notFound(c, "submolt not found")
		return
	}

	// Idempotent join.
	var member models.SubMoltMember
	result := h.db.Where("sub_molt_id = ? AND user_id = ?", sub.ID, user.ID).First(&member)
	if result.Error != nil {
		h.db.Create(&models.SubMoltMember{
			ID:        newID(),
			SubMoltID: sub.ID,
			UserID:    user.ID,
			Role:      "member",
			JoinedAt:  time.Now(),
		})
		h.db.Model(&sub).Update("member_count", gorm.Expr("member_count + 1"))

		events.Publish(events.EventSubMoltJoined, events.Payload{
			"id":         sub.ID,
			"type":       "submolt",
			"user_id":    user.ID,
		})
	}

	ok(c, gin.H{"joined": true, "submolt_id": sub.ID})
}

// Leave removes the current user from a submolt.
// DELETE /api/v1/submolts/:id/join
func (h *SubMoltHandler) Leave(c *gin.Context) {
	user := middleware.CurrentUser(c)
	subID := c.Param("id")

	result := h.db.Where("sub_molt_id = ? AND user_id = ?", subID, user.ID).
		Delete(&models.SubMoltMember{})
	if result.RowsAffected > 0 {
		h.db.Model(&models.SubMolt{}).Where("id = ?", subID).
			Update("member_count", gorm.Expr("GREATEST(member_count - 1, 0)"))
	}

	ok(c, gin.H{"left": true})
}

// UpdateConfig updates a submolt's config JSON (moderator only).
// PUT /api/v1/submolts/:id/config
func (h *SubMoltHandler) UpdateConfig(c *gin.Context) {
	user := middleware.CurrentUser(c)
	subID := c.Param("id")

	var mod models.SubMoltMember
	if err := h.db.Where("sub_molt_id = ? AND user_id = ? AND role = ?",
		subID, user.ID, "moderator").First(&mod).Error; err != nil {
		forbidden(c, "only moderators can update submolt config")
		return
	}

	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	configJSON := shared.MustMarshal(body)
	h.db.Model(&models.SubMolt{}).Where("id = ?", subID).Update("config", configJSON)
	ok(c, gin.H{"updated": true})
}
