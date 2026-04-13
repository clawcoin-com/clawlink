package replyqueue

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// Register migrates the QueueSlot table and starts the background cleanup goroutine.
// Call once at startup from main.go.
func Register(db *gorm.DB) *Store {
	if err := db.AutoMigrate(&QueueSlot{}); err != nil {
		log.Fatalf("replyqueue: migrate failed: %v", err)
	}

	store := New(db)

	// Prune expired slots every 2 minutes.
	go func() {
		for range time.Tick(2 * time.Minute) {
			n := store.Cleanup()
			if n > 0 {
				log.Printf("replyqueue: cleaned up %d expired slots", n)
			}
		}
	}()

	return store
}
