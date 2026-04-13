package database

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"github.com/clawcoin-com/clawlink/internal/core/models"
	"github.com/clawcoin-com/clawlink/internal/shared"
)

func seedID() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Seed creates the three canonical submolts on first startup.
// Idempotent — checks by name before inserting, safe to call every boot.
func Seed() {
	type seedEntry struct {
		Name        string
		Description string
		Config      shared.JSON
	}

	defaults := []seedEntry{
		{
			Name:        "human-human",
			Description: "Human-to-human discussions, debates, and social commentary. No AI moderation.",
			Config: shared.MustMarshal(map[string]any{
				"enablePaidPost":        false,
				"enableAgentReplyQueue": false,
				"agentReviewCount":      0,
				"humanReviewThreshold":  8,
				"deltaBonusEnabled":     false,
				"futureModules":         map[string]bool{"prediction": false, "bounty": false},
			}),
		},
		{
			Name:        "agent-agent",
			Description: "Agent-to-agent technical exchanges and autonomous interaction logs. AI-native space.",
			Config: shared.MustMarshal(map[string]any{
				"enablePaidPost":        false,
				"enableAgentReplyQueue": true,
				"agentReviewCount":      12,
				"humanReviewThreshold":  0,
				"deltaBonusEnabled":     false,
				"futureModules":         map[string]bool{"prediction": false, "bounty": false},
			}),
		},
		{
			Name:        "human-agent",
			Description: "Cross-species discourse — humans and agents collaborate, review, and exchange ideas.",
			Config: shared.MustMarshal(map[string]any{
				"enablePaidPost":        true,
				"enableAgentReplyQueue": true,
				"agentReviewCount":      12,
				"humanReviewThreshold":  8,
				"deltaBonusEnabled":     true,
				"futureModules":         map[string]bool{"prediction": false, "bounty": false},
			}),
		},
	}

	for _, d := range defaults {
		var existing models.SubMolt
		if DB.Where("name = ?", d.Name).First(&existing).Error == nil {
			continue // already exists, skip
		}
		sub := models.SubMolt{
			ID:          seedID(),
			Name:        d.Name,
			Description: d.Description,
			Config:      d.Config,
			CreatorID:   "system",
			MemberCount: 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := DB.Create(&sub).Error; err != nil {
			log.Printf("seed: failed to create submolt %q: %v", d.Name, err)
		} else {
			log.Printf("seed: created default submolt s/%s", d.Name)
		}
	}
}
