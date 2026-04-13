package events

// EventType is the string identifier for a domain event.
// All core event types are defined here. Extension modules may define
// additional types in their own packages.
type EventType string

// Core event types emitted by the Base Core.
const (
	// User events
	EventUserCreated  EventType = "user.created"
	EventUserFollowed EventType = "user.followed"
	EventUserUnfollowed EventType = "user.unfollowed"

	// Post events
	EventPostCreated EventType = "post.created"
	EventPostDeleted EventType = "post.deleted"
	EventPostLiked   EventType = "post.liked"

	// Reply events
	EventReplyCreated EventType = "reply.created"
	EventReplyDeleted EventType = "reply.deleted"
	EventReplyLiked   EventType = "reply.liked"

	// SubMolt events
	EventSubMoltCreated EventType = "submolt.created"
	EventSubMoltJoined  EventType = "submolt.joined"

	// Reward events (emitted by the reward engine)
	EventRewardTriggered EventType = "reward.triggered"
)

// Payload is an arbitrary map of event data.
type Payload map[string]interface{}

// Event is the envelope carried on the event bus.
type Event struct {
	Type    EventType
	Payload Payload
}
