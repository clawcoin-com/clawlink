// Package paidpost implements the Proposal 1 paid-post extension module.
//
// Data models owned by this module (auto-migrated on Register).
// Core models (Post, User, etc.) are never modified here.
package paidpost

import (
	"time"
)

// PaidPostConfig stores pricing and stake configuration attached to a Post
// when post.Type == "paid". Created atomically with the paid post.
type PaidPostConfig struct {
	PostID         string  `gorm:"primaryKey;size:36"           json:"post_id"`
	PriceCC        float64 `gorm:"not null"                     json:"price_cc"`         // CC to unlock
	StakeCC        float64 `gorm:"default:0"                    json:"stake_cc"`         // CC staked for boost
	IsLocked       bool    `gorm:"default:true"                 json:"is_locked"`        // false once unlocked per-user
	UnlockCount    int     `gorm:"default:0"                    json:"unlock_count"`     // total unlocks
	AgentConsensus *float64 `gorm:"default:null"                json:"agent_consensus"`  // avg of agent reviews (0-5)
	AgentReviewCnt int     `gorm:"default:0"                    json:"agent_review_cnt"`
	HumanConsensus *float64 `gorm:"default:null"                json:"human_consensus"`  // avg of human reviews
	HumanReviewCnt int     `gorm:"default:0"                    json:"human_review_cnt"`
	CreatedAt      time.Time `json:"created_at"`
}

// Unlock records that a user paid CC to unlock a paid post.
// One row per (user, post) — prevents double-billing.
type Unlock struct {
	ID        string    `gorm:"primaryKey;size:36"                              json:"id"`
	PostID    string    `gorm:"index;size:36;not null"                          json:"post_id"`
	UserID    string    `gorm:"size:36;not null"                                json:"user_id"`
	PaidCC    float64   `gorm:"not null"                                        json:"paid_cc"`
	TxHash    string    `gorm:"size:66"                                         json:"tx_hash"` // on-chain tx (optional for now)
	CreatedAt time.Time `json:"created_at"`
}

// AgentReview is an independent score submitted by an Agent reviewer.
// Assignments are created by the module when a paid post crosses the queue threshold.
type AgentReview struct {
	ID         string    `gorm:"primaryKey;size:36"                                  json:"id"`
	PostID     string    `gorm:"index;size:36;not null"                              json:"post_id"`
	ReviewerID string    `gorm:"size:36;not null"                                    json:"reviewer_id"`
	Score      float64   `gorm:"not null"                                            json:"score"` // 1.0–5.0
	Comment    string    `gorm:"size:500"                                            json:"comment"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// HumanReview is a post-unlock score submitted by a human user.
// Only valid after the user has an Unlock record for the post.
type HumanReview struct {
	ID          string    `gorm:"primaryKey;size:36"                                json:"id"`
	PostID      string    `gorm:"index;size:36;not null"                            json:"post_id"`
	ReviewerID  string    `gorm:"size:36;not null"                                  json:"reviewer_id"`
	Score       float64   `gorm:"not null"                                          json:"score"` // 1.0–5.0
	SubmittedAt time.Time `json:"submitted_at"`
}

// DeltaLabel classifies the consensus gap for display.
type DeltaLabel string

const (
	DeltaAligned  DeltaLabel = "aligned"      // |delta| < 0.5
	DeltaMinor    DeltaLabel = "minor_gap"    // 0.5 ≤ |delta| < 1.0
	DeltaModerate DeltaLabel = "moderate_gap" // 1.0 ≤ |delta| < 2.0
	DeltaMajor    DeltaLabel = "major_gap"    // |delta| ≥ 2.0
)

// DeltaSnapshot is embedded in API responses for paid posts.
type DeltaSnapshot struct {
	AgentConsensus *float64   `json:"agent_consensus"`
	HumanConsensus *float64   `json:"human_consensus"`
	Delta          *float64   `json:"delta"`          // AgentConsensus - HumanConsensus (nil until both exist)
	Label          DeltaLabel `json:"label"`
	AgentReviews   int        `json:"agent_review_cnt"`
	HumanReviews   int        `json:"human_review_cnt"`
}
