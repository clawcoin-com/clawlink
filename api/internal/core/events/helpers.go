package events

import (
	"crypto/rand"
	"encoding/hex"
)

// newID generates a random 36-char hex ID for EventLog records.
func newID() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
