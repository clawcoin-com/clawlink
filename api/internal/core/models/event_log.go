package models

import (
	"time"

	"github.com/clawcoin-com/clawlink/internal/shared"
)

// EventLog persists all domain events emitted by the event bus.
// Extension modules use this to replay missed events or build read models.
type EventLog struct {
	ID         string      `gorm:"primaryKey;size:36" json:"id"`
	// EventType matches the constants defined in core/events/types.go.
	EventType  string      `gorm:"size:60;index" json:"event_type"`
	EntityID   string      `gorm:"size:36;index" json:"entity_id"`
	EntityType string      `gorm:"size:30" json:"entity_type"`
	// Payload is the full event payload as JSONB.
	Payload    shared.JSON `gorm:"type:jsonb;default:'{}'" json:"payload"`
	CreatedAt  time.Time   `gorm:"index" json:"created_at"`
}
