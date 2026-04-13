// Package reward implements the RewardRule execution engine.
//
// The engine listens to the event bus and, for each event, looks up matching
// active RewardRule records in the database. When a rule matches, it executes
// the configured action (e.g. minting CC, adjusting karma).
//
// MVP: only karma adjustment is implemented. CC on-chain minting is a TODO
// pending TipContract deployment on ClawCoin Testnet.
//
// Extension modules add new rules by inserting rows into the reward_rules table.
// No core code changes are required.
package reward

import (
	"encoding/json"
	"log"

	"github.com/clawcoin-com/clawlink/internal/core/events"
	"gorm.io/gorm"
)

// Engine reads RewardRule records from DB and executes them on event receipt.
type Engine struct {
	db *gorm.DB
}

// New creates a reward engine and registers it with the global event bus.
// Call this after events.Init() and database.Connect().
func New(db *gorm.DB) *Engine {
	e := &Engine{db: db}

	// Subscribe to all core events; the engine decides which rules apply.
	coreEvents := []events.EventType{
		events.EventPostCreated,
		events.EventPostLiked,
		events.EventReplyCreated,
		events.EventReplyLiked,
		events.EventRewardTriggered,
	}
	for _, et := range coreEvents {
		et := et // capture
		events.Subscribe(et, func(ev events.Event) {
			e.handle(ev)
		})
	}

	return e
}

type rewardAction struct {
	Type   string `json:"type"`
	Amount string `json:"amount,omitempty"`
	To     string `json:"to,omitempty"` // "author", "actor", "reviewer"
}

func (e *Engine) handle(ev events.Event) {
	type ruleRow struct {
		ID     string
		Action []byte
	}
	var rules []ruleRow
	err := e.db.Table("reward_rules").
		Where("trigger_event = ? AND is_active = true", string(ev.Type)).
		Select("id, action").
		Scan(&rules).Error
	if err != nil {
		log.Printf("[RewardEngine] DB error: %v", err)
		return
	}

	for _, r := range rules {
		var action rewardAction
		if err := json.Unmarshal(r.Action, &action); err != nil {
			log.Printf("[RewardEngine] bad action JSON for rule %s: %v", r.ID, err)
			continue
		}
		e.execute(ev, action)
	}
}

func (e *Engine) execute(ev events.Event, action rewardAction) {
	switch action.Type {
	case "karma_increment":
		e.adjustKarma(ev, 1)
	case "karma_decrement":
		e.adjustKarma(ev, -1)
	case "mint_cc":
		// TODO: implement on-chain CC minting via TipContract once deployed.
		log.Printf("[RewardEngine] mint_cc action pending TipContract deployment (amount=%s to=%s)",
			action.Amount, action.To)
	default:
		log.Printf("[RewardEngine] unknown action type: %s", action.Type)
	}
}

func (e *Engine) adjustKarma(ev events.Event, delta int) {
	authorID, ok := ev.Payload["author_id"].(string)
	if !ok {
		return
	}
	e.db.Exec("UPDATE users SET karma = karma + ? WHERE id = ?", delta, authorID)
}
