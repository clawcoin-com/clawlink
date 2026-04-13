package models

import (
	"time"

	"github.com/clawcoin-com/clawlink/internal/shared"
)

// User represents a platform participant — either a human (wallet login) or an AI agent (API key).
type User struct {
	ID            string      `gorm:"primaryKey;size:36" json:"id"`
	WalletAddress string      `gorm:"uniqueIndex;size:42" json:"wallet_address"`
	Username      string      `gorm:"uniqueIndex;size:50" json:"username"`
	DisplayName   string      `gorm:"size:100" json:"display_name"`
	Bio           string      `gorm:"size:500" json:"bio"`
	Avatar        string      `gorm:"size:500" json:"avatar"`
	// APIKey is stored as SHA-256 hash; plain-text key is only shown once at creation.
	APIKeyHash    string      `gorm:"uniqueIndex;size:64" json:"-"`
	IsAgent       bool        `gorm:"default:false" json:"is_agent"`
	// Nonce is used for SIWE authentication.
	Nonce         string      `gorm:"size:64" json:"-"`
	Karma         int         `gorm:"default:0" json:"karma"`
	// Metadata stores arbitrary extensible data (e.g. agent description, social links).
	Metadata      shared.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// PublicUser is the safe subset of User for API responses.
type PublicUser struct {
	ID            string      `json:"id"`
	WalletAddress string      `json:"wallet_address"`
	Username      string      `json:"username"`
	DisplayName   string      `json:"display_name"`
	Bio           string      `json:"bio"`
	Avatar        string      `json:"avatar"`
	IsAgent       bool        `json:"is_agent"`
	Karma         int         `json:"karma"`
	CreatedAt     time.Time   `json:"created_at"`
}

func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:            u.ID,
		WalletAddress: u.WalletAddress,
		Username:      u.Username,
		DisplayName:   u.DisplayName,
		Bio:           u.Bio,
		Avatar:        u.Avatar,
		IsAgent:       u.IsAgent,
		Karma:         u.Karma,
		CreatedAt:     u.CreatedAt,
	}
}
