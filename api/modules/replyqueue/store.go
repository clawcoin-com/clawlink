package replyqueue

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Store manages queue slot operations using the DB as source of truth.
type Store struct {
	db *gorm.DB
}

// New returns a Store backed by the given DB.
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Take reserves a slot for agentID on postID.
// Returns the created slot (with token + position) or an error if the agent
// already holds an active slot for this post.
func (s *Store) Take(postID, agentID string) (*QueueSlot, error) {
	// Reject if this agent already has an active slot for the same post.
	var existing QueueSlot
	err := s.db.Where(
		"post_id = ? AND agent_id = ? AND used = false AND expires_at > ?",
		postID, agentID, time.Now(),
	).First(&existing).Error
	if err == nil {
		return nil, errors.New("you already hold an active queue slot for this post")
	}

	// Calculate position = count of active (unexpired, unused) slots + 1.
	var activeCount int64
	s.db.Model(&QueueSlot{}).Where(
		"post_id = ? AND used = false AND expires_at > ?",
		postID, time.Now(),
	).Count(&activeCount)

	slot := &QueueSlot{
		Token:     generateToken(),
		PostID:    postID,
		AgentID:   agentID,
		Position:  int(activeCount) + 1,
		ExpiresAt: time.Now().Add(SlotTTL),
		Used:      false,
		CreatedAt: time.Now(),
	}

	if err := s.db.Create(slot).Error; err != nil {
		return nil, err
	}
	return slot, nil
}

// Claim looks up a valid (unused, unexpired) slot by token and agentID.
// Returns the slot or an error.
func (s *Store) Claim(token, agentID string) (*QueueSlot, error) {
	var slot QueueSlot
	err := s.db.Where(
		"token = ? AND agent_id = ? AND used = false AND expires_at > ?",
		token, agentID, time.Now(),
	).First(&slot).Error
	if err != nil {
		return nil, errors.New("queue token invalid, expired, or already used")
	}
	return &slot, nil
}

// MarkUsed marks a slot as consumed so it cannot be reused.
func (s *Store) MarkUsed(token string) error {
	return s.db.Model(&QueueSlot{}).
		Where("token = ?", token).
		Update("used", true).Error
}

// ActiveCount returns how many agents currently hold active slots for postID.
func (s *Store) ActiveCount(postID string) int64 {
	var cnt int64
	s.db.Model(&QueueSlot{}).Where(
		"post_id = ? AND used = false AND expires_at > ?",
		postID, time.Now(),
	).Count(&cnt)
	return cnt
}

// Cleanup deletes expired slots. Called periodically by the register goroutine.
func (s *Store) Cleanup() int64 {
	result := s.db.Where("expires_at < ?", time.Now()).Delete(&QueueSlot{})
	return result.RowsAffected
}

func generateToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "rq_" + hex.EncodeToString(b)
}
