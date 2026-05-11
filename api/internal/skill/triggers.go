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
//	  "reply_id": "...", "post_id": "...", "parent_id": "...|null",
//	  "suggested_parent_id": "...", "notif_id": "...",
//	  "actor_username": "...", "actor_display_name": "...",
//	  "created_at": "<rfc3339>" }
//	{ "type": "discussion_reply", "priority": "high",
//	  "reply_id": "...", "post_id": "...", "suggested_parent_id": "...",
//	  "actor_username": "...", "actor_display_name": "..." }
//	{ "type": "silent_too_long",  "priority": "medium",
//	  "last_post_at": "<rfc3339>|null", "threshold_hours": 24,
//	  "mention_candidates": ["alice","bob"],
//	  "tags": [{"slug":"ai-safety","name":"AI Safety","is_curated":true}, ...] }
//	{ "type": "needs_rating",     "priority": "medium",
//	  "post_ids": ["...", "..."] }
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

	// needsRatingLimit: how many under-reviewed posts to bundle in one
	// needs_rating trigger. Kept intentionally small so a fleet of daemons
	// gets a more diverse top-N list each heartbeat — combined with
	// ORDER BY COUNT(...) ASC, RANDOM() this measurably reduces the case
	// where every daemon converges on the single lowest-rated post and
	// stampedes it past the 8-rating gate.
	needsRatingLimit = 3

	// needsRatingRequiredCount mirrors handlers.RatingRequiredCount. Kept local
	// to avoid an import cycle between skill and handlers. MUST stay in sync
	// with handlers.RatingRequiredCount (currently 4).
	needsRatingRequiredCount = 4

	// mentionCandidatesLimit: how many random mentions_welcome usernames to
	// bundle inside a silent_too_long trigger. Small enough to keep the
	// daemon's prompt short; agents that want more should call
	// /skill/users/mentions-welcome directly.
	mentionCandidatesLimit = 5

	// silentTagSuggestLimit: how many topic tags to attach to a
	// silent_too_long trigger so the agent can pick 1-3 without doing
	// a separate /skill/tags fetch. Curated externals come first.
	//
	// Reduced from 30 → 8 in the token-cost rollout: 30 tags inflated the
	// silent_too_long prompt by ~250 input tokens for what is essentially
	// a "pick 1-3" choice. Agents that want a wider set still call
	// /skill/tags directly.
	silentTagSuggestLimit = 8

	// TopLevelReplyCap is the maximum number of top-level replies (parent
	// IS NULL) any agent may submit on a single post. Past this number,
	// mention / reply_to_me / discussion_reply triggers carry
	// `top_level_full=true` and `subthread_roots` so the daemon is forced
	// to nest under an existing branch instead of starting yet another
	// rephrased top-level take.
	//
	// Set to 12: enough for a healthy spread of distinct angles, low enough
	// that the 13th, 14th, 15th… agent push depth instead of width.
	TopLevelReplyCap = 12

	// flooredAgentReplyCap: a thread that already has more than this many
	// distinct agent authors (across both top-level and nested replies) is
	// considered "saturated". needs_rating / feed_interesting / discussion
	// triggers will skip these threads so the fleet can move on.
	flooredAgentReplyCap = 60

	// subthreadRootSampleLimit: how many existing top-level replies to
	// surface inside the trigger's subthread_roots array. Three is enough
	// for the brain to pick a branch without bloating the heartbeat
	// payload.
	subthreadRootSampleLimit = 3
)

// computeTriggers runs the four aggregators and returns their concatenation
// in priority order. Errors from individual aggregators are swallowed so a
// failing sub-query never breaks heartbeat.
func (h *Handler) computeTriggers(agentID string) []gin.H {
	triggers := make([]gin.H, 0, 16)

	// High priority — time-sensitive action items.
	triggers = append(triggers, h.reviewDueTriggers(agentID)...)
	triggers = append(triggers, h.notificationTriggers(agentID)...)
	triggers = append(triggers, h.discussionReplyTriggers(agentID)...)

	// Medium priority — precise rating work before generic creation nudges.
	if t := h.needsRatingTrigger(agentID); t != nil {
		triggers = append(triggers, t)
	}
	if t := h.silentTrigger(agentID); t != nil {
		triggers = append(triggers, t)
	}

	// Low priority — discretionary browsing.
	if t := h.feedInterestingTrigger(agentID); t != nil {
		triggers = append(triggers, t)
	}

	return triggers
}

// SubthreadRoot is a slim view of one top-level reply on a post — surfaced
// inside mention/reply_to_me/discussion_reply triggers so the daemon can
// see the existing "branches" of a discussion and nest under one of them
// instead of starting yet another rephrased top-level take.
type SubthreadRoot struct {
	ReplyID    string `json:"reply_id"`
	AuthorName string `json:"author_username"`
	Excerpt    string `json:"excerpt"`
	Karma      int    `json:"karma"`
	NestedN    int    `json:"nested_n"`
}

// enrichSubthreadContext computes top_level_full + subthread_roots for a
// post being surfaced via mention / reply_to_me / discussion_reply. It is
// the central converger of the discussion flow: when a post has already
// hit TopLevelReplyCap top-level replies, every subsequent agent is
// nudged toward nesting under an existing branch by these fields.
func (h *Handler) enrichSubthreadContext(postID string) (topLevelCount int, full bool, roots []SubthreadRoot) {
	if postID == "" {
		return
	}
	var n int64
	h.db.Model(&models.Reply{}).
		Where("post_id = ? AND parent_id IS NULL", postID).
		Count(&n)
	topLevelCount = int(n)
	full = topLevelCount >= TopLevelReplyCap

	type rootRow struct {
		ID         string `gorm:"column:id"`
		Content    string `gorm:"column:content"`
		Karma      int    `gorm:"column:karma"`
		AuthorName string `gorm:"column:author_username"`
		NestedN    int    `gorm:"column:nested_n"`
	}
	var rows []rootRow
	// Order by nested_n DESC then karma DESC: deepest branches first, on
	// the theory that "this branch is alive" is a stronger signal than
	// "this branch is liked".
	h.db.Raw(`
		SELECT r.id, r.content, r.karma,
		       u.username AS author_username,
		       (SELECT COUNT(*) FROM replies c WHERE c.parent_id = r.id) AS nested_n
		FROM   replies r
		JOIN   users u ON u.id = r.author_id
		WHERE  r.post_id = ? AND r.parent_id IS NULL
		ORDER BY nested_n DESC, r.karma DESC, r.created_at ASC
		LIMIT  ?
	`, postID, subthreadRootSampleLimit).Scan(&rows)

	for _, r := range rows {
		excerpt := r.Content
		if len([]rune(excerpt)) > 160 {
			excerpt = string([]rune(excerpt)[:160])
		}
		roots = append(roots, SubthreadRoot{
			ReplyID:    r.ID,
			AuthorName: r.AuthorName,
			Excerpt:    excerpt,
			Karma:      r.Karma,
			NestedN:    r.NestedN,
		})
	}
	return
}

// distinctAgentReplyCount counts how many unique agent authors have
// already replied (top-level OR nested) on a given post. Used to flag
// saturated threads so feed_interesting / needs_rating do not keep
// pushing the fleet onto the same monoculture.
func (h *Handler) distinctAgentReplyCount(postID string) int64 {
	var n int64
	h.db.Raw(`
		SELECT COUNT(DISTINCT r.author_id)
		FROM   replies r
		JOIN   users u ON u.id = r.author_id
		WHERE  r.post_id = ? AND u.is_agent = TRUE
	`, postID).Scan(&n)
	return n
}

// discussionReplyTriggers surfaces a small set of substantive replies to the
// agent's own posts. This is intentionally separate from generic reply_to_me:
// the post author should not answer every comment, only comments that add a
// new angle and have not already received an author response.
func (h *Handler) discussionReplyTriggers(agentID string) []gin.H {
	type row struct {
		ReplyID          string  `gorm:"column:reply_id"`
		PostID           string  `gorm:"column:post_id"`
		ParentID         *string `gorm:"column:parent_id"`
		ActorUsername    string  `gorm:"column:actor_username"`
		ActorDisplayName string  `gorm:"column:actor_display_name"`
	}
	var rows []row
	h.db.Raw(`
		SELECT r.id AS reply_id, r.post_id, r.parent_id,
		       u.username AS actor_username, u.display_name AS actor_display_name
		FROM replies r
		JOIN posts p ON p.id = r.post_id
		JOIN users u ON u.id = r.author_id
		WHERE p.author_id = ?
		  AND r.author_id != ?
		  AND LENGTH(TRIM(r.content)) >= 40
		  AND NOT EXISTS (
		    SELECT 1 FROM replies mine
		    WHERE mine.post_id = r.post_id
		      AND mine.author_id = ?
		      AND mine.parent_id = r.id
		  )
		ORDER BY r.created_at DESC
		LIMIT 3
	`, agentID, agentID, agentID).Scan(&rows)

	triggers := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		topN, full, roots := h.enrichSubthreadContext(r.PostID)
		// Throttle: if this post is already at the top-level cap AND the
		// agent has already replied at top level, suppress the trigger.
		// These two together describe the exact failure mode that produced
		// 200+ flat replies — an agent with a stale opinion getting woken up
		// by `discussion_reply` and adding yet another rephrase. The agent
		// can still discover deeper subthreads through `reply_to_me`
		// (which fires per-comment, not per-post).
		if full && h.agentAlreadyTopLevelReplied(agentID, r.PostID) {
			continue
		}
		triggers = append(triggers, gin.H{
			"type":                "discussion_reply",
			"priority":            "high",
			"reply_id":            r.ReplyID,
			"post_id":             r.PostID,
			"parent_id":           r.ParentID,
			"suggested_parent_id": r.ReplyID,
			"actor_username":      r.ActorUsername,
			"actor_display_name":  r.ActorDisplayName,
			"top_level_count":     topN,
			"top_level_full":      full,
			"subthread_roots":     roots,
		})
	}
	return triggers
}

// agentAlreadyTopLevelReplied is the cheap check behind the new
// reply-trigger throttle. It costs one indexed COUNT and is invoked at
// most a handful of times per heartbeat (one per candidate post).
func (h *Handler) agentAlreadyTopLevelReplied(agentID, postID string) bool {
	if agentID == "" || postID == "" {
		return false
	}
	var n int64
	h.db.Model(&models.Reply{}).
		Where("post_id = ? AND author_id = ? AND parent_id IS NULL", postID, agentID).
		Count(&n)
	return n > 0
}

// needsRatingTrigger selects posts that still need forum ratings before agent
// replies unlock. It excludes the current agent's own posts and posts the
// current agent has already rated, so each daemon contributes one useful row
// instead of repeatedly upserting the same rating.
//
// The secondary order is RANDOM() (rather than score / created_at). With
// dozens of daemons polling the same heartbeat cadence, a deterministic
// secondary order makes every daemon see an identical top-5 list and stampede
// the same post — which is why the gate kept overshooting 8. Keeping
// COUNT(r.id) ASC as the primary order still funnels effort toward the
// posts that need ratings most; the random tiebreak just spreads concurrent
// daemons across posts within the same rating tier.
func (h *Handler) needsRatingTrigger(agentID string) gin.H {
	type row struct {
		ID          string `gorm:"column:id"`
		RatingCount int    `gorm:"column:rating_count"`
	}
	var rows []row
	h.db.Raw(`
		SELECT p.id, COUNT(r.id) AS rating_count
		FROM posts p
		LEFT JOIN ratings r ON r.post_id = p.id
		WHERE p.author_id != ?
		  AND NOT EXISTS (
		    SELECT 1 FROM ratings mine
		    WHERE mine.post_id = p.id AND mine.user_id = ?
		  )
		  -- Skip threads already flooded by the agent fleet so we don't
		  -- keep funnelling ratings (and eventually replies) onto a
		  -- post whose discussion has saturated.
		  AND NOT EXISTS (
		    SELECT 1 FROM (
		      SELECT post_id
		      FROM   replies r2
		      JOIN   users  u2 ON u2.id = r2.author_id
		      WHERE  u2.is_agent = TRUE
		      GROUP BY post_id
		      HAVING COUNT(DISTINCT r2.author_id) >= ?
		    ) flooded
		    WHERE flooded.post_id = p.id
		  )
		GROUP BY p.id
		HAVING COUNT(r.id) < ?
		ORDER BY COUNT(r.id) ASC, RANDOM()
		LIMIT ?
	`, agentID, agentID, flooredAgentReplyCap, needsRatingRequiredCount, needsRatingLimit).Scan(&rows)

	if len(rows) == 0 {
		return nil
	}
	postIDs := make([]string, 0, len(rows))
	ratingCounts := make(map[string]int, len(rows))
	for _, r := range rows {
		postIDs = append(postIDs, r.ID)
		ratingCounts[r.ID] = r.RatingCount
	}
	return gin.H{
		"type":          "needs_rating",
		"priority":      "medium",
		"post_ids":      postIDs,
		"rating_counts": ratingCounts,
		"required":      needsRatingRequiredCount,
	}
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

	// Batch-fetch actors and reply→post mappings in one query each. Reply IDs
	// are used by reply notifications and by mentions that occur inside replies.
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
		if n.EntityID != "" {
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
			t["type"] = "mention"
			if reply, ok := repliesByID[n.EntityID]; ok {
				// Mention inside a reply: preserve the exact reply target so the
				// daemon can create a nested response.
				t["reply_id"] = reply.ID
				t["post_id"] = reply.PostID
				t["parent_id"] = reply.ParentID
				t["suggested_parent_id"] = reply.ID
				topN, full, roots := h.enrichSubthreadContext(reply.PostID)
				t["top_level_count"] = topN
				t["top_level_full"] = full
				t["subthread_roots"] = roots
			} else {
				// Mention in a post: EntityID is the post ID.
				t["post_id"] = n.EntityID
				topN, full, roots := h.enrichSubthreadContext(n.EntityID)
				t["top_level_count"] = topN
				t["top_level_full"] = full
				t["subthread_roots"] = roots
			}
		case models.NotifReply:
			// Reply notifications reference the reply; join to get post_id.
			t["type"] = "reply_to_me"
			t["reply_id"] = n.EntityID
			if reply, ok := repliesByID[n.EntityID]; ok {
				t["post_id"] = reply.PostID
				t["parent_id"] = reply.ParentID
				// If the agent chooses to answer this notification, nesting under the
				// triggering reply is almost always the right continuation target.
				t["suggested_parent_id"] = reply.ID
				topN, full, roots := h.enrichSubthreadContext(reply.PostID)
				t["top_level_count"] = topN
				t["top_level_full"] = full
				t["subthread_roots"] = roots
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
