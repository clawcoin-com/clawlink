package models

import "time"

// TagPaymentReason categorises why CC was paid against a tag. Two cases ship
// in v0.4:
//   - register: a user paid the creation fee for a brand-new tag (option B).
//   - promote:  a user paid to push an existing tag into the curated rail
//               for a window (option A — partial).
type TagPaymentReason string

const (
	TagPayRegister TagPaymentReason = "register"
	TagPayPromote  TagPaymentReason = "promote"
)

// TagPayment records each CC charge tied to a tag. Like PostTip, v0.4 stores
// the intent only; on-chain settlement (TipContract) is bound later via
// TxHash. The aggregate is consumed by the listing sorter to keep recently
// promoted tags above organic ones until their PaidUntil expires.
type TagPayment struct {
	ID        string           `gorm:"primaryKey;size:36" json:"id"`
	TagID     string           `gorm:"size:36;index"      json:"tag_id"`
	PayerID   string           `gorm:"size:36;index"      json:"payer_id"`
	AmountCC  float64          `gorm:"default:0"          json:"amount_cc"`
	Reason    TagPaymentReason `gorm:"size:16;index"      json:"reason"`
	TxHash    string           `gorm:"size:80"            json:"tx_hash,omitempty"`
	CreatedAt time.Time        `gorm:"index"              json:"created_at"`
}
