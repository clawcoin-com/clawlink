package events

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/shared"
	"gorm.io/gorm"
)

// Handler is a function that processes an event.
type Handler func(e Event)

// Bus is an in-memory, goroutine-safe publish/subscribe event bus.
// For production scale, swap the internal channel/map for Watermill + NATS/RabbitMQ
// without changing any caller code — just replace this implementation.
type Bus struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler
	ch       chan Event
	db       *gorm.DB
}

var Global *Bus

// Init creates and starts the global event bus.
func Init(db *gorm.DB) {
	Global = &Bus{
		handlers: make(map[EventType][]Handler),
		ch:       make(chan Event, 256),
		db:       db,
	}
	go Global.process()
}

// Subscribe registers a handler for the given event type.
// Handlers are called asynchronously in the order they were registered.
// Safe to call from multiple goroutines.
func Subscribe(eventType EventType, h Handler) {
	if Global == nil {
		log.Println("[EventBus] warning: Subscribe called before Init")
		return
	}
	Global.mu.Lock()
	defer Global.mu.Unlock()
	Global.handlers[eventType] = append(Global.handlers[eventType], h)
}

// Publish emits an event to all subscribers and persists it to the EventLog table.
// Non-blocking: events are queued internally.
func Publish(eventType EventType, payload Payload) {
	if Global == nil {
		return
	}
	Global.ch <- Event{Type: eventType, Payload: payload}
}

// process is the internal dispatch loop — runs in a dedicated goroutine.
func (b *Bus) process() {
	for e := range b.ch {
		b.persist(e)
		b.dispatch(e)
	}
}

func (b *Bus) dispatch(e Event) {
	b.mu.RLock()
	handlers := b.handlers[e.Type]
	b.mu.RUnlock()

	for _, h := range handlers {
		h(e)
	}
}

func (b *Bus) persist(e Event) {
	raw, _ := json.Marshal(e.Payload)
	entityID, _ := e.Payload["id"].(string)
	entityType, _ := e.Payload["type"].(string)

	entry := &models.EventLog{
		ID:         newID(),
		EventType:  string(e.Type),
		EntityID:   entityID,
		EntityType: entityType,
		Payload:    shared.JSON(raw),
		CreatedAt:  time.Now(),
	}
	if result := b.db.Create(entry); result.Error != nil {
		// Non-fatal: log and continue.
		log.Printf("[EventBus] failed to persist event %s: %v", e.Type, result.Error)
	}
}
