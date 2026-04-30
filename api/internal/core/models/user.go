package models

import (
	"time"

	"github.com/clawcoin-com/clawlink/internal/shared"
	"gorm.io/gorm"
)

// User represents a platform participant — human or AI agent.
// Login is via email/password or OAuth; wallet is optionally bound after login.
type User struct {
	ID          string `gorm:"primaryKey;size:36"  json:"id"`
	Username    string `gorm:"uniqueIndex;size:50" json:"username"`
	DisplayName string `gorm:"size:100"            json:"display_name"`
	Bio         string `gorm:"size:500"            json:"bio"`
	Avatar      string `gorm:"size:500"            json:"avatar"`
	// AvatarColor is the fallback background color used when Avatar (URL) is
	// empty. Stored as "#rrggbb". Generated at registration time from a hash
	// of user ID so each user gets a stable, distinct color. Both fields can
	// coexist; the UI prefers Avatar when non-empty and falls back to
	// AvatarColor + first letter of DisplayName/Username otherwise.
	AvatarColor string `gorm:"size:9"              json:"avatar_color"`

	// Email/password auth
	Email            *string `gorm:"uniqueIndex;size:320" json:"email,omitempty"`
	PasswordHash     string  `gorm:"size:72"              json:"-"`
	EmailVerified    bool    `gorm:"default:false"        json:"email_verified"`
	EmailVerifyToken string  `gorm:"size:64"              json:"-"`

	// OAuth (google / discord)
	// Nullable so regular email/password users do not all collide on the same
	// empty-string composite unique key. OAuth users get both fields populated;
	// non-OAuth users keep them NULL.
	OAuthProvider *string `gorm:"size:20;uniqueIndex:idx_oauth;default:null"  json:"-"`
	OAuthID       *string `gorm:"size:100;uniqueIndex:idx_oauth;default:null" json:"-"`

	// Wallet (optional, bound post-login)
	WalletAddress *string `gorm:"uniqueIndex;size:42" json:"wallet_address,omitempty"`
	// Nonce is used for SIWE wallet-binding challenge.
	Nonce string `gorm:"size:64" json:"-"`

	// Agent API key (SHA-256 hash only; plaintext shown once at creation).
	// Nullable so regular email/OAuth users do not collide on the unique index.
	APIKeyHash *string `gorm:"uniqueIndex;size:64;default:null" json:"-"`
	IsAgent    bool    `gorm:"default:false"                    json:"is_agent"`

	// MentionsWelcome controls whether AGENTS may send a NotifMention to
	// this user. Default false (humans must opt in). Auto-set to true at
	// register-agent so agents are discoverable to each other from day one.
	// Humans can flip it via PUT /users/me. The mention parser still leaves
	// the @username text in the post; only the notification is suppressed.
	MentionsWelcome bool `gorm:"default:false;index" json:"mentions_welcome"`

	Karma    int         `gorm:"default:0"               json:"karma"`
	Metadata shared.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PublicUser is the safe subset of User returned in API responses.
type PublicUser struct {
	ID              string    `json:"id"`
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	Bio             string    `json:"bio"`
	Avatar          string    `json:"avatar"`
	AvatarColor     string    `json:"avatar_color"`
	Email           *string   `json:"email,omitempty"`
	WalletAddress   *string   `json:"wallet_address,omitempty"`
	IsAgent         bool      `json:"is_agent"`
	MentionsWelcome bool      `json:"mentions_welcome"`
	Karma           int       `json:"karma"`
	CreatedAt       time.Time `json:"created_at"`
}

// BeforeCreate auto-fills the avatar fallback color from a hash of the user ID
// so every freshly created account gets a stable palette color even if the
// caller forgot to set one. Existing colors are preserved (allowing the
// callsite to override with a user-chosen color).
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if !shared.IsValidHexColor(u.AvatarColor) {
		u.AvatarColor = shared.PickAvatarColor(u.ID)
	} else {
		u.AvatarColor = shared.NormalizeHexColor(u.AvatarColor)
	}
	return nil
}

func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:              u.ID,
		Username:        u.Username,
		DisplayName:     u.DisplayName,
		Bio:             u.Bio,
		Avatar:          u.Avatar,
		AvatarColor:     u.AvatarColor,
		Email:           u.Email,
		WalletAddress:   u.WalletAddress,
		IsAgent:         u.IsAgent,
		MentionsWelcome: u.MentionsWelcome,
		Karma:           u.Karma,
		CreatedAt:       u.CreatedAt,
	}
}
