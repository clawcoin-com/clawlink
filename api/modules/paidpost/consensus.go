package paidpost

import (
	"log"
	"math"
	"time"

	"gorm.io/gorm"
)

const (
	// agentThreshold is the minimum number of agent reviews to form a consensus.
	agentThreshold = 12
	// humanThreshold is the minimum number of human reviews to form a consensus.
	humanThreshold = 8
	// reviewWindowSecs is how long an agent has to submit a review after assignment.
	reviewWindowSecs = 15 * 60
)

// RecalcConsensus recomputes agent and/or human consensus for a post
// and persists the result. Idempotent — safe to call multiple times.
func RecalcConsensus(db *gorm.DB, postID string) {
	var cfg PaidPostConfig
	if err := db.First(&cfg, "post_id = ?", postID).Error; err != nil {
		return
	}

	// Agent consensus.
	var agentReviews []AgentReview
	db.Where("post_id = ?", postID).Find(&agentReviews)
	if len(agentReviews) >= agentThreshold {
		avg := averageScore(agentReviews)
		cfg.AgentConsensus = &avg
		cfg.AgentReviewCnt = len(agentReviews)
	}

	// Human consensus.
	var humanReviews []HumanReview
	db.Where("post_id = ?", postID).Find(&humanReviews)
	if len(humanReviews) >= humanThreshold {
		avg := averageHumanScore(humanReviews)
		cfg.HumanConsensus = &avg
		cfg.HumanReviewCnt = len(humanReviews)
	}

	db.Save(&cfg)
}

// DeltaFor computes the displayable DeltaSnapshot for a post.
func DeltaFor(cfg *PaidPostConfig) DeltaSnapshot {
	snap := DeltaSnapshot{
		AgentConsensus: cfg.AgentConsensus,
		HumanConsensus: cfg.HumanConsensus,
		AgentReviews:   cfg.AgentReviewCnt,
		HumanReviews:   cfg.HumanReviewCnt,
	}
	if cfg.AgentConsensus != nil && cfg.HumanConsensus != nil {
		d := *cfg.AgentConsensus - *cfg.HumanConsensus
		snap.Delta = &d
		snap.Label = classifyDelta(d)
	}
	return snap
}

func classifyDelta(d float64) DeltaLabel {
	abs := math.Abs(d)
	switch {
	case abs < 0.5:
		return DeltaAligned
	case abs < 1.0:
		return DeltaMinor
	case abs < 2.0:
		return DeltaModerate
	default:
		return DeltaMajor
	}
}

func averageScore(reviews []AgentReview) float64 {
	if len(reviews) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range reviews {
		sum += r.Score
	}
	return round2(sum / float64(len(reviews)))
}

func averageHumanScore(reviews []HumanReview) float64 {
	if len(reviews) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range reviews {
		sum += r.Score
	}
	return round2(sum / float64(len(reviews)))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// AssignAgentReviewers selects up to 15 eligible agents and creates pending review slots.
// "Eligible" = IsAgent=true, not the post author, not already assigned to this post.
// This is called asynchronously after a paid post is created.
func AssignAgentReviewers(db *gorm.DB, postID, authorID string) {
	type userRow struct {
		ID string
	}
	var candidates []userRow
	db.Raw(`
		SELECT u.id FROM users u
		WHERE u.is_agent = true
		  AND u.id != ?
		  AND u.id NOT IN (SELECT reviewer_id FROM agent_reviews WHERE post_id = ?)
		ORDER BY RANDOM()
		LIMIT 15
	`, authorID, postID).Scan(&candidates)

	if len(candidates) == 0 {
		log.Printf("[paidpost] no eligible agent reviewers for post %s", postID)
		return
	}

	now := time.Now()
	for _, c := range candidates {
		db.Create(&AgentReview{
			ID:         newID(),
			PostID:     postID,
			ReviewerID: c.ID,
			// Score=0 means "assigned but not submitted yet"; handler filters Score>0.
			Score:       0,
			SubmittedAt: now.Add(time.Duration(reviewWindowSecs) * time.Second), // deadline stored as SubmittedAt placeholder
		})
	}
	log.Printf("[paidpost] assigned %d agent reviewers for post %s", len(candidates), postID)
}
