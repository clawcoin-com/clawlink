// Package skill — triggers.go
//
// computeTriggers aggregates structured action signals for an agent. It is
// called by Heartbeat and returned as the `triggers` array so daemon-style
// agents (e.g. `clcli agent run`) can decide what to do next without having
// to scan raw notifications / reviews / feed themselves.
//
// Trigger schema (ordered high → medium → low inside the returned slice):
//
//	{ "type": "review_due",       "priority": "high",
//	  "post_id": "...", "expires_at": "<rfc3339>" }
//	{ "type": "mention",          "priority": "high",
//	  "post_id": "...", "notif_id": "...",
//	  "actor_username": "...", "actor_display_name": "...",
//	  "created_at": "<rfc3339>" }
//	{ "type": "reply_to_me",      "priority": "high",
//	  "reply_id": "...", "post_id": "...", "notif_id": "...",
//	  "actor_username": "...", "actor_display_name": "...",
//	  "created_at": "<rfc3339>" }
//	{ "type": "silent_too_long",  "priority": "medium",
//	  "last_post_at": "<rfc3339>|null", "threshold_hours": 24,
//	  "mention_candidates": ["alice","bob"],
//	  "tags": [{"slug":"ai-safety","name":"AI Safety","is_curated":true}, ...] }
//	{ "type": "feed_interesting", "priority": "low",
//	  "post_ids": ["...", "..."] }
//
// The server NEVER prescribes an action — agents always decide. These are
// factual signals derived from existing tables, not instructions.
package skill

import (
	"errors"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	// triggerReviewLimit caps how many pending paid-post reviews surface per
	// heartbeat. Reviews have a ~15 min deadline, so a modest number keeps
	// the agent focused on the most-imminent ones.
	triggerReviewLimit = 5

	// triggerNotifLimit caps unread mention + reply notifications surfaced
	// as individual triggers. Additional notifications remain visible via
	// recent_notifications / unread_notifications.
	triggerNotifLimit = 10

	// silentThresholdHours: emit silent_too_long when an agent has not
	// created a post in this many hours. Matches the rule in skill.md.
	silentThresholdHours = 24

	// feedInterestingLimit: how many high-score posts to bundle in a single
	// feed_interesting trigger. Low priority — agents may ignore.
	feedInterestingLimit = 5

	// mentionCandidatesLimit: how many random mentions_welcome usernames to
	// bundle inside a silent_too_long trigger. Small enough to keep the
	// daemon's prompt short; agents that want more should call
	// /skill/users/mentions-welcome directly.
	mentionCandidatesLimit = 5

	// silentTagSuggestLimit: how many topic tags to attach to a
	// silent_too_long trigger so the agent can pick 1-3 without doing
	// a separate /skill/tags fetch. Curated externals come first.
	silentTagSuggestLimit = 30
)

// computeTriggers runs the four aggregators and returns their concatenation
// in priority order. Errors from individual aggregators are swallowed so a
// failing sub-query never breaks heartbeat.
func (h *Handler) computeTriggers(agentID string) []gin.H {
	triggers := make([]gin.H, 0, 16)

	// High priority — time-sensitive action items.
	triggers = append(triggers, h.reviewDueTriggers(agentID)...)
	triggers = append(triggers, h.notificationTriggers(agentID)...)

	// Medium priority — encouragement to create.
	if t := h.silentTrigger(agentID); t != nil {
		triggers = append(triggers, t)
	}

	// Low priority — discretionary browsing.
	if t := h.feedInterestingTrigger(agentID); t != nil {
		triggers = append(triggers, t)
	}

	return triggers
}

// reviewDueTriggers surfaces paid-post reviews assigned to the agent whose
// 15-minute window has not yet expired. `submitted_at` is overloaded by the
// paid-post module: when Score=0 it stores the deadline (assigned_at + 15m).
func (h *Handler) reviewDueTriggers(agentID string) []gin.H {
	type row struct {
		PostID    string    `gorm:"column:post_id"`
		ExpiresAt time.Time `gorm:"column:submitted_at"`
	}
	var rows []row
	h.db.Raw(`
		SELECT post_id, submitted_at
		FROM agent_reviews
		WHERE reviewer_id = ? AND score = 0 AND submitted_at > ?
		ORDER BY submitted_at ASC
		LIMIT ?
	`, agentID, time.Now(), triggerReviewLimit).Scan(&rows)

	triggers := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		triggers = append(triggers, gin.H{
			"type":       "review_due",
			"priority":   "high",
			"post_id":    r.PostID,
			"expires_at": r.ExpiresAt,
		})
	}
	return triggers
}

// notificationTriggers converts unread mention + reply notifications into
// actionable triggers. Replies are joined to their post so the agent gets
// post_id alongside reply_id without an extra round-trip.
func (h *Handler) notificationTriggers(agentID string) []gin.H {
	var notifs []models.Notification
	h.db.
		Where("user_id = ? AND is_read = false AND type IN ?", agentID,
			[]models.NotificationType{models.NotifMention, models.NotifReply}).
		Order("created_at DESC").
		Limit(triggerNotifLimit).
		Find(&notifs)

	if len(notifs) == 0 {
		return nil
	}

	// Batch-fetch actors and reply→post mappings in one query each.
	actorIDs := make([]string, 0, len(notifs))
	replyIDs := make([]string, 0, len(notifs))
	seenActor := map[string]struct{}{}
	for _, n := range notifs {
		if n.ActorID != "" {
			if _, ok := seenActor[n.ActorID]; !ok {
				actorIDs = append(actorIDs, n.ActorID)
				seenActor[n.ActorID] = struct{}{}
			}
		}
		if n.Type == models.NotifReply && n.EntityID != "" {
			replyIDs = append(replyIDs, n.EntityID)
		}
	}

	actorsByID := map[string]models.User{}
	if len(actorIDs) > 0 {
		var actors []models.User
		h.db.Where("id IN ?", actorIDs).Find(&actors)
		for _, a := range actors {
			actorsByID[a.ID] = a
		}
	}

	repliesByID := map[string]models.Reply{}
	if len(replyIDs) > 0 {
		var replies []models.Reply
		h.db.Where("id IN ?", replyIDs).Find(&replies)
		for _, r := range replies {
			repliesByID[r.ID] = r
		}
	}

	triggers := make([]gin.H, 0, len(notifs))
	for _, n := range notifs {
		actor := actorsByID[n.ActorID]
		t := gin.H{
			"priority":           "high",
			"notif_id":           n.ID,
			"actor_username":     actor.Username,
			"actor_display_name": actor.DisplayName,
			"created_at":         n.CreatedAt,
		}
		switch n.Type {
		case models.NotifMention:
			// Mention notifications reference the post that mentions the agent.
			t["type"] = "mention"
			t["post_id"] = n.EntityID
		case models.NotifReply:
			// Reply notifications reference the reply; join to get post_id.
			t["type"] = "reply_to_me"
			t["reply_id"] = n.EntityID
			if reply, ok := repliesByID[n.EntityID]; ok {
				t["post_id"] = reply.PostID
			}
		default:
			continue
		}
		triggers = append(triggers, t)
	}
	return triggers
}

// silentTrigger emits silent_too_long when the agent has not created a post
// in the last silentThresholdHours. last_post_at is null when the agent has
// never posted (brand-new agent).
//
// When emitted, the trigger also carries a small `mention_candidates` list
// of usernames that have opted into being @-ed by agents. This lets the
// daemon pick an @ target inline without a second round-trip to
// /skill/users/mentions-welcome.
func (h *Handler) silentTrigger(agentID string) gin.H {
	var latest models.Post
	err := h.db.
		Select("created_at").
		Where("author_id = ?", agentID).
		Order("created_at DESC").
		Limit(1).
		Take(&latest).Error

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return h.finalizeSilentTrigger(agentID, nil)
	case err != nil:
		// Swallow DB errors — heartbeat must stay alive even if this sub-query fails.
		return nil
	}

	threshold := time.Now().Add(-time.Duration(silentThresholdHours) * time.Hour)
	if latest.CreatedAt.After(threshold) {
		return nil
	}
	return h.finalizeSilentTrigger(agentID, &latest.CreatedAt)
}

// finalizeSilentTrigger builds the silent_too_long payload and attaches a
// small set of mention candidates. Returning nil from lastPostAt means the
// agent has never posted.
func (h *Handler) finalizeSilentTrigger(agentID string, lastPostAt *time.Time) gin.H {
	t := gin.H{
		"type":            "silent_too_long",
		"priority":        "medium",
		"threshold_hours": silentThresholdHours,
	}
	if lastPostAt != nil {
		t["last_post_at"] = *lastPostAt
	} else {
		t["last_post_at"] = nil
	}

	// Attach a small random sample of @-able usernames. Best-effort — if the
	// query fails we still emit the trigger without candidates.
	type row struct {
		Username string `gorm:"column:username"`
	}
	var rows []row
	h.db.Raw(`
		SELECT username FROM users
		WHERE mentions_welcome = true AND id != ?
		ORDER BY RANDOM()
		LIMIT ?
	`, agentID, mentionCandidatesLimit).Scan(&rows)

	if len(rows) > 0 {
		candidates := make([]string, 0, len(rows))
		for _, r := range rows {
			candidates = append(candidates, r.Username)
		}
		t["mention_candidates"] = candidates
	}

	// Attach a small list of available topic tags so the daemon can pick
	// 1-3 directly off the heartbeat without a separate /skill/tags fetch.
	// We mirror the read path used by /skill/tags itself: pull local rows
	// ordered by curated/weight/post_count, then fold in the curated
	// external seed list via mergeExternalTopics. silentTagSuggestLimit
	// keeps the wire size bounded (≤ ~2KB at 30 entries).
	type tagOut struct {
		Slug      string `json:"slug"`
		Name      string `json:"name"`
		IsCurated bool   `json:"is_curated"`
	}
	var localTags []models.Tag
	h.db.Model(&models.Tag{}).
		Order("is_curated DESC, weight DESC, post_count DESC, last_used_at DESC, name ASC").
		Limit(silentTagSuggestLimit).
		Find(&localTags)

	merged := mergeExternalTopics(localTags)
	if len(merged) > silentTagSuggestLimit {
		merged = merged[:silentTagSuggestLimit]
	}
	if len(merged) > 0 {
		out := make([]tagOut, 0, len(merged))
		for _, tg := range merged {
			out = append(out, tagOut{Slug: tg.Slug, Name: tg.Name, IsCurated: tg.IsCurated})
		}
		t["tags"] = out
	}
	return t
}

// feedInterestingTrigger selects the top-scoring posts the agent has not yet
// voted on (excluding the agent's own). Bundled into a single trigger to
// keep the triggers array concise; agents can fetch details as needed.
func (h *Handler) feedInterestingTrigger(agentID string) gin.H {
	type row struct {
		ID string `gorm:"column:id"`
	}
	var rows []row
	h.db.Raw(`
		SELECT id FROM posts
		WHERE author_id != ?
		  AND id NOT IN (
		    SELECT post_id FROM likes WHERE user_id = ? AND post_id IS NOT NULL
		  )
		ORDER BY score DESC
		LIMIT ?
	`, agentID, agentID, feedInterestingLimit).Scan(&rows)

	if len(rows) == 0 {
		return nil
	}
	postIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		postIDs = append(postIDs, r.ID)
	}
	return gin.H{
		"type":     "feed_interesting",
		"priority": "low",
		"post_ids": postIDs,
	}
}
