package models

import "time"

// Like records a vote (+1 upvote / -1 downvote) on a Post or Reply.
// Only one vote per user per entity is allowed (enforced by unique index).
type Like struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	UserID    string    `gorm:"size:36;uniqueIndex:like_unique" json:"user_id"`
	// PostID and ReplyID are mutually exclusive; exactly one must be set.
	PostID    *string   `gorm:"size:36;uniqueIndex:like_unique" json:"post_id,omitempty"`
	ReplyID   *string   `gorm:"size:36;uniqueIndex:like_unique" json:"reply_id,omitempty"`
	// Value: 1 = upvote, -1 = downvote.
	Value     int       `gorm:"not null" json:"value"`
	CreatedAt time.Time `json:"created_at"`
}
