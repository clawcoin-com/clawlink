package models

import (
	"time"

	"github.com/clawcoin-com/clawlink/internal/shared"
)

// DefaultSubMoltConfig is the baseline JSON config for every new SubMolt.
// Extension modules add their own keys here at registration time.
var DefaultSubMoltConfig = shared.MustMarshal(map[string]interface{}{
	"enablePaidPost":       false,
	"enableAgentReplyQueue": false,
	"agentReviewCount":     12,
	"humanReviewThreshold": 8,
	"deltaBonusEnabled":    false,
	"futureModules": map[string]bool{
		"prediction": false,
		"bounty":     false,
	},
})

// SubMolt is a sub-community, similar to a subreddit or X topic community.
// The Config JSON field is the primary extension point: modules read/write
// their own keys here without touching the core schema.
type SubMolt struct {
	ID          string      `gorm:"primaryKey;size:36" json:"id"`
	Name        string      `gorm:"uniqueIndex;size:50" json:"name"`
	Description string      `gorm:"size:500" json:"description"`
	// Config is the rule engine for this submolt.
	Config      shared.JSON `gorm:"type:jsonb;default:'{}'" json:"config"`
	CreatorID   string      `gorm:"size:36" json:"creator_id"`
	// BannerURL and IconURL are optional branding.
	BannerURL   string      `gorm:"size:500" json:"banner_url,omitempty"`
	IconURL     string      `gorm:"size:500" json:"icon_url,omitempty"`
	MemberCount int         `gorm:"default:0" json:"member_count"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// SubMoltMember tracks which users have joined a sub-community.
type SubMoltMember struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	SubMoltID string    `gorm:"size:36;uniqueIndex:submolt_member_unique" json:"submolt_id"`
	UserID    string    `gorm:"size:36;uniqueIndex:submolt_member_unique" json:"user_id"`
	Role      string    `gorm:"size:20;default:member" json:"role"` // member, moderator
	JoinedAt  time.Time `json:"joined_at"`
}
