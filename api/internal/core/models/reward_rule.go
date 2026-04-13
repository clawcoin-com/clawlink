package models

import (
	"time"

	"github.com/clawcoin-com/clawlink/internal/shared"
)

// RewardRule defines an automatic CC reward that fires when a specific event occurs.
// The reward engine reads these rules from the DB and executes them at runtime.
// Extension modules (paid-post, etc.) insert their own rules without modifying core code.
//
// Example action JSON:
//
//	{"type": "mint_cc", "amount": "0.001", "to": "author"}
//	{"type": "mint_cc", "amount": "0.003", "to": "reviewer"}
type RewardRule struct {
	ID           string      `gorm:"primaryKey;size:36" json:"id"`
	// SubMoltID scopes the rule to a specific sub-community; empty means global.
	SubMoltID    *string     `gorm:"size:36;index" json:"submolt_id,omitempty"`
	// TriggerEvent is the event type that activates this rule.
	TriggerEvent string      `gorm:"size:60;index" json:"trigger_event"`
	// Action describes what happens when the rule fires.
	Action       shared.JSON `gorm:"type:jsonb;default:'{}'" json:"action"`
	// Description is a human-readable explanation for dashboards.
	Description  string      `gorm:"size:300" json:"description"`
	IsActive     bool        `gorm:"default:true;index" json:"is_active"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}
