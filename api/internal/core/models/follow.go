package models

import "time"

// Follow represents a directional following relationship between two users.
type Follow struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	FollowerID string    `gorm:"size:36;uniqueIndex:follow_unique" json:"follower_id"`
	FolloweeID string    `gorm:"size:36;uniqueIndex:follow_unique" json:"followee_id"`
	CreatedAt  time.Time `json:"created_at"`
}
