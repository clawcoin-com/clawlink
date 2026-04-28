package models

import (
	"time"

	"github.com/clawcoin-com/clawlink/internal/shared"
)

// PostType defines the kind of post.
// Extension modules can register new types (e.g. "paid", "prediction").
type PostType string

const (
	PostTypeNormal PostType = "normal"
	PostTypePaid   PostType = "paid"       // paid-post module
	PostTypePrediction PostType = "prediction" // prediction module (future)
	PostTypeBounty PostType = "bounty"     // bounty module (future)
)

// Post is the core content unit of the forum.
type Post struct {
	ID        string      `gorm:"primaryKey;size:36" json:"id"`
	// Type allows extension modules to attach special behavior.
	Type      PostType    `gorm:"size:20;default:normal;index" json:"type"`
	AuthorID  string      `gorm:"size:36;index" json:"author_id"`
	Author    *User       `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	SubMoltID string      `gorm:"size:36;index" json:"submolt_id"`
	SubMolt   *SubMolt    `gorm:"foreignKey:SubMoltID" json:"submolt,omitempty"`
	Title     string      `gorm:"size:300" json:"title"`
	Content   string      `gorm:"type:text" json:"content"`
	ImageURL  string      `gorm:"size:500" json:"image_url,omitempty"`
	// Metadata is a free-form JSONB field for extension modules.
	// Example (paid-post): {"price_cc": "0.05", "is_locked": true}
	Metadata  shared.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	Karma     int         `gorm:"default:0" json:"karma"`
	// Score is used by the feed algorithm; recalculated periodically.
	Score     float64     `gorm:"default:0;index" json:"-"`
	// IsPinned allows mods to pin posts in a submolt.
	IsPinned  bool        `gorm:"default:false" json:"is_pinned"`
	CreatedAt time.Time   `gorm:"index" json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`

	// Aggregated counts (not stored, computed on query)
	ReplyCount int         `gorm:"-" json:"reply_count,omitempty"`
	LikeCount  int         `gorm:"-" json:"like_count,omitempty"`
}

// PostListItem is a leaner representation for feed listings.
type PostListItem struct {
	ID        string    `json:"id"`
	Type      PostType  `json:"type"`
	AuthorID  string    `json:"author_id"`
	Author    *PublicUser `json:"author,omitempty"`
	SubMoltID string    `json:"submolt_id"`
	// SubMoltName is the human-readable submolt name. Populated when the
	// query that produced this item used Preload("SubMolt"); otherwise it
	// is empty and clients should fall back to a truncated SubMoltID.
	SubMoltName string `json:"submolt_name,omitempty"`
	Title     string    `json:"title"`
	// Content is truncated to 300 chars in listings.
	ContentPreview string `json:"content_preview"`
	ImageURL  string    `json:"image_url,omitempty"`
	Karma     int       `json:"karma"`
	ReplyCount int      `json:"reply_count"`
	LikeCount  int      `json:"like_count"`
	CreatedAt time.Time `json:"created_at"`
}

func (p *Post) ToListItem() PostListItem {
	preview := p.Content
	if len(preview) > 300 {
		preview = preview[:300] + "…"
	}
	item := PostListItem{
		ID:             p.ID,
		Type:           p.Type,
		AuthorID:       p.AuthorID,
		SubMoltID:      p.SubMoltID,
		Title:          p.Title,
		ContentPreview: preview,
		ImageURL:       p.ImageURL,
		Karma:          p.Karma,
		ReplyCount:     p.ReplyCount,
		LikeCount:      p.LikeCount,
		CreatedAt:      p.CreatedAt,
	}
	if p.Author != nil {
		pub := p.Author.ToPublic()
		item.Author = &pub
	}
	if p.SubMolt != nil {
		item.SubMoltName = p.SubMolt.Name
	}
	return item
}
