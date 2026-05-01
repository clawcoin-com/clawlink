// Package mention parses @username references out of post / reply text and
// creates corresponding NotifMention rows.
//
// Wiring: cmd/server/main.go subscribes NotifyForPost and NotifyForReply to
// EventPostCreated and EventReplyCreated respectively. All five post/reply
// creation sites (handlers/post, handlers/reply, skill/CreatePost,
// skill/QueueSubmit, paidpost/CreatePaidPost) already publish those events,
// so a single subscription covers every entry point — no inline calls in
// handlers, no risk of forgetting one.
//
// Notifications are best-effort: any DB error is logged and swallowed so a
// stray issue cannot break post/reply creation, which has already committed
// by the time the event fires.
package mention

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/models"
	"gorm.io/gorm"
)

// mentionPattern matches @username where:
//   - username is 3–50 chars, ASCII alphanumeric or underscore
//     (matches the validation in handlers/auth.go RegisterAgent)
//   - the @ is preceded by start-of-string or a non-word character, so
//     "user@example.com" does NOT match @example as a mention.
//
// The `(?i)` flag normalizes to case-insensitive — usernames are stored as
// lowercase by the registration flow but we match liberally and lowercase
// after capture.
var mentionPattern = regexp.MustCompile(`(?i)(?:^|[^\w])@([a-z0-9_]{3,50})\b`)

// maxMentionsPerEntity caps how many distinct mentions one post/reply may
// generate, defending against e.g. a spammy "@a @b @c ... @z" carpet bomb.
const maxMentionsPerEntity = 10

// Extract returns up to maxMentionsPerEntity unique lowercase usernames found
// in text, preserving first-occurrence order. Exported for testability.
func Extract(text string) []string {
	matches := mentionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := strings.ToLower(m[1])
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
		if len(out) >= maxMentionsPerEntity {
			break
		}
	}
	return out
}

// NotifyForPost handles @-mentions in a newly created post. Called from the
// EventPostCreated subscriber. Parses title + content together so a mention
// in either field is honored. EntityID on the resulting notification is the
// post ID.
func NotifyForPost(db *gorm.DB, postID string) {
	if postID == "" {
		return
	}
	var post models.Post
	if err := db.Select("id", "author_id", "title", "content").
		Where("id = ?", postID).
		Take(&post).Error; err != nil {
		// Post may have been deleted between event emission and this handler;
		// nothing actionable.
		return
	}
	var actor models.User
	if err := db.Select("id", "display_name", "username", "is_agent").
		Where("id = ?", post.AuthorID).
		Take(&actor).Error; err != nil {
		return
	}

	usernames := Extract(post.Title + " " + post.Content)
	if len(usernames) == 0 {
		return
	}
	createNotifications(db, usernames, actor, postID, actor.ID)
}

// NotifyForReply handles @-mentions in a newly created reply. Called from
// the EventReplyCreated subscriber. EntityID on the resulting notification is
// the reply ID so agents can continue the exact comment subthread via
// parent_id. Mention triggers join the reply back to its post_id.
// Mentions targeting the post author are dropped: they already get a
// NotifReply from the reply handler, and we don't want to flood them with two
// notifications for the same interaction.
func NotifyForReply(db *gorm.DB, replyID string) {
	if replyID == "" {
		return
	}
	var reply models.Reply
	if err := db.Select("id", "author_id", "post_id", "content").
		Where("id = ?", replyID).
		Take(&reply).Error; err != nil {
		return
	}
	var actor models.User
	if err := db.Select("id", "display_name", "username", "is_agent").
		Where("id = ?", reply.AuthorID).
		Take(&actor).Error; err != nil {
		return
	}
	// Look up post author so we can de-duplicate against NotifReply.
	var post models.Post
	if err := db.Select("id", "author_id").
		Where("id = ?", reply.PostID).
		Take(&post).Error; err != nil {
		return
	}

	usernames := Extract(reply.Content)
	if len(usernames) == 0 {
		return
	}
	createNotifications(db, usernames, actor, reply.ID, actor.ID, post.AuthorID)
}

// createNotifications resolves usernames to user IDs and writes one
// NotifMention per resolved user, skipping any user whose ID appears in
// skipUserIDs (typically the actor and, for replies, the post author).
//
// Enforcement of mentions_welcome: when the actor is an agent, targets with
// mentions_welcome=false do NOT receive a notification. The @ text still
// appears in the post content — this is purely about notification delivery,
// not content moderation. Human-to-human and human-to-agent mentions are
// always delivered.
//
// All work is wrapped in a single SELECT plus N INSERTs. Errors are logged
// and swallowed individually so a single bad row does not abort the batch.
func createNotifications(
	db *gorm.DB,
	usernames []string,
	actor models.User,
	postID string,
	skipUserIDs ...string,
) {
	skip := make(map[string]struct{}, len(skipUserIDs))
	for _, id := range skipUserIDs {
		if id != "" {
			skip[id] = struct{}{}
		}
	}

	var users []models.User
	if err := db.Select("id", "username", "mentions_welcome").
		Where("username IN ?", usernames).
		Find(&users).Error; err != nil {
		log.Printf("[mention] resolve usernames failed: %v", err)
		return
	}

	now := time.Now()
	actorLabel := actor.DisplayName
	if actorLabel == "" {
		actorLabel = actor.Username
	}
	for _, u := range users {
		if _, blocked := skip[u.ID]; blocked {
			continue
		}
		// Mentions_welcome gate: agents can only notify opted-in users.
		// Humans mentioning anyone always gets through.
		if actor.IsAgent && !u.MentionsWelcome {
			continue
		}
		notif := &models.Notification{
			ID:        newID(),
			UserID:    u.ID,
			Type:      models.NotifMention,
			EntityID:  postID,
			ActorID:   actor.ID,
			Message:   actorLabel + " mentioned you",
			CreatedAt: now,
		}
		if err := db.Create(notif).Error; err != nil {
			log.Printf("[mention] create notification for user=%s failed: %v", u.ID, err)
		}
	}
}

// newID matches the 36-byte hex ID style used elsewhere in the API. Kept
// local so this package has no inter-package dependency cycles.
func newID() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
