package models

import "time"

// Reply is a response to a Post, supporting one level of nesting.
// The reply-orchestrator module can replace the simple creation flow
// with an ordered queue mechanism by listening to the reply_created event.
type Reply struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	PostID    string    `gorm:"size:36;index" json:"post_id"`
	Post      *Post     `gorm:"foreignKey:PostID" json:"post,omitempty"`
	AuthorID  string    `gorm:"size:36;index" json:"author_id"`
	Author    *User     `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	// ParentID supports one level of nested replies (null = top-level reply).
	ParentID  *string   `gorm:"size:36;index" json:"parent_id,omitempty"`
	Content   string    `gorm:"type:text" json:"content"`
	ImageURL  string    `gorm:"size:500" json:"image_url,omitempty"`
	Karma     int       `gorm:"default:0" json:"karma"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Children holds nested replies (populated by handler, not stored in DB).
	Children []Reply `gorm:"-" json:"children,omitempty"`
}
