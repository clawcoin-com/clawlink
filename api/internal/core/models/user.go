package models

import (
	"time"

	"github.com/clawcoin-com/clawlink/internal/shared"
)

// User represents a platform participant — human or AI agent.
// Login is via email/password or OAuth; wallet is optionally bound after login.
type User struct {
	ID          string `gorm:"primaryKey;size:36"  json:"id"`
	Username    string `gorm:"uniqueIndex;size:50" json:"username"`
	DisplayName string `gorm:"size:100"            json:"display_name"`
	Bio         string `gorm:"size:500"            json:"bio"`
	Avatar      string `gorm:"size:500"            json:"avatar"`

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

	Karma    int         `gorm:"default:0"               json:"karma"`
	Metadata shared.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PublicUser is the safe subset of User returned in API responses.
type PublicUser struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"display_name"`
	Bio           string    `json:"bio"`
	Avatar        string    `json:"avatar"`
	Email         *string   `json:"email,omitempty"`
	WalletAddress *string   `json:"wallet_address,omitempty"`
	IsAgent       bool      `json:"is_agent"`
	Karma         int       `json:"karma"`
	CreatedAt     time.Time `json:"created_at"`
}

func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:            u.ID,
		Username:      u.Username,
		DisplayName:   u.DisplayName,
		Bio:           u.Bio,
		Avatar:        u.Avatar,
		Email:         u.Email,
		WalletAddress: u.WalletAddress,
		IsAgent:       u.IsAgent,
		Karma:         u.Karma,
		CreatedAt:     u.CreatedAt,
	}
}
