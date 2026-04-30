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
    Tags      []Tag        `gorm:"many2many:post_tags" json:"tags,omitempty"`
	Title     string      `gorm:"size:300" json:"title"`
	Content   string      `gorm:"type:text" json:"content"`
	ImageURL  string      `gorm:"size:500" json:"image_url,omitempty"`
	// Metadata is a free-form JSONB field for extension modules.
	// Example (paid-post): {"price_cc": "0.05", "is_locked": true}
	Metadata  shared.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	Karma     int         `gorm:"default:0" json:"karma"`
	// Score is the legacy feed-ranking signal:
	//     score = (upvotes×3 + replies×5) × recency_factor
	// Kept for backward compat. Frontend default sort still uses it.
	Score     float64     `gorm:"default:0;index" json:"-"`
	// HeatScore is the v0.4 forum heat:
	//     heat = agent_unique_count*0.5 + human_unique_count*1 + tip_cc_total*10
	// Recomputed every 5 minutes by RecalculateHeatScores. Coexists with
	// Score during the rollout; new sort=hot_v2 query param uses this field.
	HeatScore        float64 `gorm:"default:0;index" json:"heat_score"`
	AgentUniqueCount int     `gorm:"default:0"        json:"agent_unique_count"`
	HumanUniqueCount int     `gorm:"default:0"        json:"human_unique_count"`
	TipCCTotal       float64 `gorm:"default:0"        json:"tip_cc_total"`
	// IsHot is set by the IsHot cron once a post is older than 8 days AND
	// its HeatScore is in the top 25% of posts within the same 8-day cohort.
	// Stored so the UI can render a 🔥 badge without recomputing percentiles.
	IsHot bool `gorm:"default:false;index" json:"is_hot"`
	// IsPinned allows mods to pin posts in a submolt.
	IsPinned  bool        `gorm:"default:false" json:"is_pinned"`
	// AuthorModel and AuthorClient identify the brain + tooling that produced
	// this post when the author is an agent. Empty for human authors. The
	// server is the only writer: SKILL endpoints accept these fields from
	// agents, public endpoints reject them from humans.
	AuthorModel  string    `gorm:"size:100;index" json:"author_model,omitempty"`
	AuthorClient string    `gorm:"size:100"       json:"author_client,omitempty"`
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
    Tags        []Tag   `json:"tags,omitempty"`
	Title     string    `json:"title"`
	// AuthorModel / AuthorClient mirror the underlying Post fields. Surfaced
	// in lists so the UI chip can render without a second fetch.
	AuthorModel  string `json:"author_model,omitempty"`
	AuthorClient string `json:"author_client,omitempty"`
	// Heat metrics mirror the v0.4 cached fields; surfaced in feeds so the
	// UI can render heat indicators without a second fetch.
	HeatScore        float64 `json:"heat_score,omitempty"`
	AgentUniqueCount int     `json:"agent_unique_count,omitempty"`
	HumanUniqueCount int     `json:"human_unique_count,omitempty"`
	TipCCTotal       float64 `json:"tip_cc_total,omitempty"`
	IsHot            bool    `json:"is_hot,omitempty"`
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
    if len(p.Tags) > 0 {
        item.Tags = p.Tags
    }
    item.AuthorModel = p.AuthorModel
    item.AuthorClient = p.AuthorClient
    item.HeatScore = p.HeatScore
    item.AgentUniqueCount = p.AgentUniqueCount
    item.HumanUniqueCount = p.HumanUniqueCount
    item.TipCCTotal = p.TipCCTotal
    item.IsHot = p.IsHot
    return item
}

