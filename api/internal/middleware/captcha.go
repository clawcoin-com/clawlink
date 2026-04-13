// Package middleware/captcha implements a lightweight math-challenge CAPTCHA
// used to slow down automated Agent registrations.
//
// Flow:
//   GET  /api/v1/auth/captcha              → { token, question }
//   POST /api/v1/auth/apikey  (body: { captcha_token, captcha_answer }) → API key
//
// Challenges expire after 10 minutes; answers are checked once then discarded.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"
)

type challenge struct {
	answer    int
	expiresAt time.Time
}

type captchaStore struct {
	mu         sync.Mutex
	challenges map[string]challenge
}

var Captcha = &captchaStore{challenges: make(map[string]challenge)}

func init() {
	// Periodic cleanup of expired challenges.
	go func() {
		for range time.Tick(5 * time.Minute) {
			Captcha.cleanup()
		}
	}()
}

// Issue generates a new math challenge and returns (token, question).
func (s *captchaStore) Issue() (token, question string) {
	a, _ := rand.Int(rand.Reader, big.NewInt(20))
	b, _ := rand.Int(rand.Reader, big.NewInt(20))
	answer := int(a.Int64() + b.Int64())
	question = fmt.Sprintf("%d + %d = ?", a.Int64(), b.Int64())

	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token = hex.EncodeToString(tokenBytes)

	s.mu.Lock()
	s.challenges[token] = challenge{answer: answer, expiresAt: time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	return
}

// Verify checks that answer matches the stored challenge for token.
// Returns false if the token is unknown, expired, or the answer is wrong.
// The challenge is consumed (deleted) on first use regardless of outcome.
func (s *captchaStore) Verify(token string, answer int) bool {
	s.mu.Lock()
	ch, ok := s.challenges[token]
	delete(s.challenges, token) // one-time use
	s.mu.Unlock()

	if !ok || time.Now().After(ch.expiresAt) {
		return false
	}
	return ch.answer == answer
}

func (s *captchaStore) cleanup() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, ch := range s.challenges {
		if now.After(ch.expiresAt) {
			delete(s.challenges, k)
		}
	}
}
