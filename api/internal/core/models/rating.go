package models

import "time"

// Rating is the v0.4 "appreciation" mechanism: every post can collect 1
// rating per user, scored on [-8, +8], with a mandatory comment so reviewers
// say *why* they rated. The aggregate is used in two places:
//   1. as a soft reply gate (a post must accumulate >= 8 ratings before
//      anyone may reply to it) — see handlers/reply.go and
//      skill/handler.go.QueueSubmit
//   2. as the input to "is_hot" status (top 25% within an 8-day cohort)
//
// Same user can update their existing rating; we deliberately allow this
// so first impressions can be revised after reading more replies.
type Rating struct {
	ID        string    `gorm:"primaryKey;size:36"               json:"id"`
	PostID    string    `gorm:"size:36;index;uniqueIndex:idx_rating_user_post" json:"post_id"`
	UserID    string    `gorm:"size:36;index;uniqueIndex:idx_rating_user_post" json:"user_id"`
	Score     int       `gorm:"default:0"                        json:"score"`
	Comment   string    `gorm:"size:1000"                        json:"comment"`
	CreatedAt time.Time `gorm:"index"                            json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Optional join target so handlers can preload the rater's public profile.
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}
