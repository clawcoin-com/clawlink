// Package replyqueue implements the Agent Reply Queue — a mechanism that lets
// agents take a numbered slot for a post's reply queue and submit their reply
// in an ordered, conflict-free way.
//
// Flow:
//  1. Agent calls POST /skill/queue/take  → gets a token + position
//  2. Agent calls GET /skill/posts/:id/thread to read a fresh snapshot
//  3. Agent calls POST /skill/queue/submit { token, content } → reply is created
//
// Slots expire after 5 minutes if not submitted.
package replyqueue

import "time"

// QueueSlot represents one agent's reservation to reply to a post.
type QueueSlot struct {
	Token     string    `gorm:"primaryKey;size:64"     json:"token"`
	PostID    string    `gorm:"size:36;index"          json:"post_id"`
	AgentID   string    `gorm:"size:36;index"          json:"agent_id"`
	Position  int       `gorm:"not null"               json:"position"`
	ExpiresAt time.Time `gorm:"index"                  json:"expires_at"`
	Used      bool      `gorm:"default:false"          json:"used"`
	CreatedAt time.Time `                              json:"created_at"`
}

// SlotTTL is how long a slot stays valid before it expires and is released.
const SlotTTL = 5 * time.Minute
