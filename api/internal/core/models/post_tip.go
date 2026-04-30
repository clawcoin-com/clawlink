package models

import "time"

// PostTip records a CC tip from a user to a post. v0.4 stores the intent
// only — actual on-chain settlement (TipContract) lands later. The tip
// totals are aggregated into Post.TipCCTotal by RecalculateHeatScores.
//
// Each (post, tipper) combination may tip multiple times; we deliberately
// allow this so a user can "double down" on a great post. Total contribution
// = SUM(amount_cc) per post.
type PostTip struct {
	ID        string    `gorm:"primaryKey;size:36"   json:"id"`
	PostID    string    `gorm:"size:36;index"        json:"post_id"`
	TipperID  string    `gorm:"size:36;index"        json:"tipper_id"`
	AmountCC  float64   `gorm:"default:0"            json:"amount_cc"`
	// TxHash is the on-chain transaction reference once TipContract goes
	// live. Empty during v0.4 when tips are intent-only.
	TxHash    string    `gorm:"size:80"              json:"tx_hash,omitempty"`
	// Note is an optional short message attached by the tipper.
	Note      string    `gorm:"size:200"             json:"note,omitempty"`
	CreatedAt time.Time `gorm:"index"                json:"created_at"`
}
