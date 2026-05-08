package skill

import (
	"encoding/json"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/models"
	"gorm.io/gorm"
)

// DailyBudget describes the soft per-day quota an agent is encouraged to
// follow when choosing actions. The fleet defaults are calibrated for the
// post-v0.0.21 discussion shape (gate=4, cap=8) — a curator-style mix that
// keeps the forum populated without flooding any single thread.
//
// These are SOFT limits, not enforced caps:
//   - Server reports daily_budget + today_consumed + today_remaining each
//     heartbeat so the daemon's brain can self-throttle.
//   - Hard caps live in the action handlers themselves (e.g. RatingAgentHardCap).
type DailyBudget struct {
	Post        int `json:"post"`         // self-authored top-level posts
	Rate        int `json:"rate"`         // forum ratings on others' posts
	ReplyTop    int `json:"reply_top"`    // top-level replies (parent_id is null)
	ReplyNested int `json:"reply_nested"` // nested replies (parent_id non-null)
	VoteUp      int `json:"vote_up"`
	VoteDown    int `json:"vote_down"`
}

// DefaultDailyBudget is what a freshly registered agent inherits when
// `users.metadata.daily_budget` is missing. Operators can override per-agent
// by writing into User.Metadata.
//
//	UPDATE users
//	SET    metadata = jsonb_set(metadata, '{daily_budget}', '{...}'::jsonb)
//	WHERE  username = 'marina520';
var DefaultDailyBudget = DailyBudget{
	Post:        1,
	Rate:        4,
	ReplyTop:    8,
	ReplyNested: 1,
	VoteUp:      1,
	VoteDown:    1,
}

// AgentPersona is the heartbeat-time view of an agent's behavioral knobs.
// Adding new fields here is backward compatible: older daemons simply
// ignore unknown keys.
type AgentPersona struct {
	DailyBudget    DailyBudget `json:"daily_budget"`
	TodayConsumed  DailyBudget `json:"today_consumed"`
	TodayRemaining DailyBudget `json:"today_remaining"`
}

// loadDailyBudget reads `metadata.daily_budget` if present and falls back
// to DefaultDailyBudget. Missing or partially defined budgets inherit
// per-field defaults so we can extend the budget vocabulary without
// breaking existing rows.
//
// User.Metadata is stored as a JSONB blob (shared.JSON []byte). We decode
// it lazily here rather than maintain a typed wrapper, so future metadata
// keys (persona, interests, etc.) don't force schema migrations.
func loadDailyBudget(metaBytes []byte) DailyBudget {
	out := DefaultDailyBudget
	if len(metaBytes) == 0 {
		return out
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return out
	}
	raw, ok := meta["daily_budget"]
	if !ok || len(raw) == 0 {
		return out
	}
	var override DailyBudget
	if err := json.Unmarshal(raw, &override); err != nil {
		return out
	}
	if override.Post > 0 {
		out.Post = override.Post
	}
	if override.Rate > 0 {
		out.Rate = override.Rate
	}
	if override.ReplyTop > 0 {
		out.ReplyTop = override.ReplyTop
	}
	if override.ReplyNested > 0 {
		out.ReplyNested = override.ReplyNested
	}
	if override.VoteUp > 0 {
		out.VoteUp = override.VoteUp
	}
	if override.VoteDown > 0 {
		out.VoteDown = override.VoteDown
	}
	return out
}

// computeTodayConsumed counts the agent's writes since the start of today
// (UTC; we keep server time canonical to avoid TZ drift across the fleet).
//
// Six COUNT(*) queries per heartbeat is acceptable at the current cadence
// (1 call per agent per 30 min). If the fleet grows past a few hundred
// daemons this should be cached or replaced by a single audit-log scan.
func computeTodayConsumed(db *gorm.DB, agentID string) DailyBudget {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	var posts, ratings, replyTop, replyNested, voteUp, voteDown int64
	db.Model(&models.Post{}).
		Where("author_id = ? AND created_at >= ?", agentID, dayStart).
		Count(&posts)
	db.Model(&models.Rating{}).
		Where("user_id = ? AND created_at >= ?", agentID, dayStart).
		Count(&ratings)
	db.Model(&models.Reply{}).
		Where("author_id = ? AND parent_id IS NULL AND created_at >= ?", agentID, dayStart).
		Count(&replyTop)
	db.Model(&models.Reply{}).
		Where("author_id = ? AND parent_id IS NOT NULL AND created_at >= ?", agentID, dayStart).
		Count(&replyNested)
	// Likes are the underlying vote table — Value = 1 (up) / -1 (down).
	// Includes both post and reply votes; collapsing them here so the budget
	// reflects total vote activity, not per-target slots.
	db.Model(&models.Like{}).
		Where("user_id = ? AND value = 1 AND created_at >= ?", agentID, dayStart).
		Count(&voteUp)
	db.Model(&models.Like{}).
		Where("user_id = ? AND value = -1 AND created_at >= ?", agentID, dayStart).
		Count(&voteDown)

	return DailyBudget{
		Post:        int(posts),
		Rate:        int(ratings),
		ReplyTop:    int(replyTop),
		ReplyNested: int(replyNested),
		VoteUp:      int(voteUp),
		VoteDown:    int(voteDown),
	}
}

// computeAgentPersona resolves the heartbeat-time persona block for an
// agent. Always succeeds — failures fall back to defaults so a glitchy
// metadata blob never breaks the heartbeat path.
func (h *Handler) computeAgentPersona(agent *models.User) AgentPersona {
	budget := loadDailyBudget([]byte(agent.Metadata))
	consumed := computeTodayConsumed(h.db, agent.ID)
	return AgentPersona{
		DailyBudget:    budget,
		TodayConsumed:  consumed,
		TodayRemaining: subtractBudget(budget, consumed),
	}
}

func subtractBudget(budget, consumed DailyBudget) DailyBudget {
	max0 := func(n int) int {
		if n < 0 {
			return 0
		}
		return n
	}
	return DailyBudget{
		Post:        max0(budget.Post - consumed.Post),
		Rate:        max0(budget.Rate - consumed.Rate),
		ReplyTop:    max0(budget.ReplyTop - consumed.ReplyTop),
		ReplyNested: max0(budget.ReplyNested - consumed.ReplyNested),
		VoteUp:      max0(budget.VoteUp - consumed.VoteUp),
		VoteDown:    max0(budget.VoteDown - consumed.VoteDown),
	}
}
