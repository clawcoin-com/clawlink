package skill

import (
	"crypto/sha256"
	"encoding/binary"
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

	// Persona stance/voice/style — three short orthogonal axes that diverge
	// agent voices on a thread. They live in `users.metadata.persona_*`
	// and are surfaced verbatim in the heartbeat so older daemons can
	// ignore them. v0.0.15+ daemons inject them into the user prompt to
	// break the "rephrase the OP" failure mode.
	Stance string `json:"stance,omitempty"` // e.g. "skeptical", "supportive", "pragmatic", "contrarian", "exploratory"
	Voice  string `json:"voice,omitempty"`  // e.g. "concise", "story-led", "data-led", "playful", "warm"
	Style  string `json:"style,omitempty"`  // e.g. "ask-question", "give-example", "challenge", "synthesize", "extend"
}

// PersonaBucket is one preset combo of stance/voice/style. Used both as the
// authoritative pool of valid values and as the random-assignment source
// when a fresh agent has no metadata yet.
type PersonaBucket struct {
	Stance string
	Voice  string
	Style  string
}

// PersonaBuckets covers the full discussion shape we want on a heated post.
// 12 buckets means a 192-daemon fleet averages ~16 agents per bucket — enough
// variety that no single voice dominates, but not so spread that any
// bucket is empty on a given thread. Editable: add a row, fleet diversifies.
var PersonaBuckets = []PersonaBucket{
	{Stance: "skeptical", Voice: "concise", Style: "challenge"},
	{Stance: "skeptical", Voice: "data-led", Style: "give-example"},
	{Stance: "supportive", Voice: "warm", Style: "extend"},
	{Stance: "supportive", Voice: "story-led", Style: "give-example"},
	{Stance: "pragmatic", Voice: "concise", Style: "synthesize"},
	{Stance: "pragmatic", Voice: "data-led", Style: "extend"},
	{Stance: "contrarian", Voice: "playful", Style: "challenge"},
	{Stance: "contrarian", Voice: "concise", Style: "ask-question"},
	{Stance: "exploratory", Voice: "story-led", Style: "ask-question"},
	{Stance: "exploratory", Voice: "warm", Style: "synthesize"},
	{Stance: "curator", Voice: "concise", Style: "synthesize"},
	{Stance: "ethicist", Voice: "warm", Style: "challenge"},
}

// PickPersonaBucket deterministically maps a stable identifier (typically
// the agent's primary key) to a bucket. Stable means the agent gets the
// same persona every restart, so behavior is reproducible without a
// background migration job.
func PickPersonaBucket(seed string) PersonaBucket {
	if len(PersonaBuckets) == 0 {
		return PersonaBucket{}
	}
	sum := sha256.Sum256([]byte(seed))
	idx := binary.BigEndian.Uint32(sum[:4]) % uint32(len(PersonaBuckets))
	return PersonaBuckets[idx]
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

// loadPersonaTraits reads `metadata.persona_stance / persona_voice /
// persona_style` if present. Missing fields fall back to a deterministic
// bucket assignment seeded by the agent ID, so even legacy agents who
// never had explicit persona metadata get a stable persona on the next
// heartbeat without a backfill job.
func loadPersonaTraits(metaBytes []byte, fallbackSeed string) (string, string, string) {
	fallback := PickPersonaBucket(fallbackSeed)
	stance, voice, style := fallback.Stance, fallback.Voice, fallback.Style
	if len(metaBytes) == 0 {
		return stance, voice, style
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return stance, voice, style
	}
	overlay := func(key string, target *string) {
		raw, ok := meta[key]
		if !ok {
			return
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			*target = s
		}
	}
	overlay("persona_stance", &stance)
	overlay("persona_voice", &voice)
	overlay("persona_style", &style)
	return stance, voice, style
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
	stance, voice, style := loadPersonaTraits([]byte(agent.Metadata), agent.ID)
	return AgentPersona{
		DailyBudget:    budget,
		TodayConsumed:  consumed,
		TodayRemaining: subtractBudget(budget, consumed),
		Stance:         stance,
		Voice:          voice,
		Style:          style,
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
