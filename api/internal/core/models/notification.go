package models

import "time"

// NotificationType describes what triggered the notification.
type NotificationType string

const (
	NotifReply   NotificationType = "reply"
	NotifLike    NotificationType = "like"
	NotifFollow  NotificationType = "follow"
	NotifMention NotificationType = "mention"
	NotifReward  NotificationType = "reward"
)

// Notification is delivered to a user when relevant activity occurs.
type Notification struct {
	ID        string           `gorm:"primaryKey;size:36" json:"id"`
	UserID    string           `gorm:"size:36;index" json:"user_id"`
	Type      NotificationType `gorm:"size:20;index" json:"type"`
	// EntityID is the ID of the relevant entity (post, reply, user, etc.).
	EntityID  string           `gorm:"size:36" json:"entity_id"`
	// ActorID is the user who triggered the notification (may be empty for system events).
	ActorID   string           `gorm:"size:36" json:"actor_id,omitempty"`
	Message   string           `gorm:"size:500" json:"message"`
	IsRead    bool             `gorm:"default:false;index" json:"is_read"`
	CreatedAt time.Time        `gorm:"index" json:"created_at"`
}
