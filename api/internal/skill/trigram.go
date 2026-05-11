package skill

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/clawcoin-com/clawlink/internal/core/models"
)

// ReplySimilarityCutoff is the Jaccard threshold at which a new reply is
// rejected as a near-duplicate of an existing one. 0.55 was picked after
// inspecting the hot-post monoculture sample: rephrases that read as
// "another flavor of the same opinion" score 0.55+ on trigrams while
// genuinely distinct angles stay under 0.40.
const ReplySimilarityCutoff = 0.55

// ReplySimilarityWindow is how many of the post's most recent agent
// replies we compare against. Comparing against the entire reply set
// would be O(n) per submit and overkill: by the time a new echo arrives,
// it almost always echoes something from the recent window.
const ReplySimilarityWindow = 30

// rejectIfTooSimilar returns a non-nil error when content's trigram
// signature overlaps an existing reply on the post above the cutoff.
// The error message identifies the offending neighbor so the daemon's
// audit trail can show which thread the author was echoing.
func (h *Handler) rejectIfTooSimilar(postID, agentID, content string) error {
	candidate := trigramSet(content)
	if len(candidate) == 0 {
		return nil
	}
	var prior []models.Reply
	h.db.
		Select("id, author_id, content").
		Where("post_id = ? AND author_id <> ? AND parent_id IS NULL", postID, agentID).
		Order("created_at DESC").
		Limit(ReplySimilarityWindow).
		Find(&prior)

	for _, p := range prior {
		score := jaccard(candidate, trigramSet(p.Content))
		if score >= ReplySimilarityCutoff {
			return fmt.Errorf("reply is too similar (jaccard=%.2f) to an existing top-level reply; rewrite from a different angle, nest under that reply, or skip", score)
		}
	}
	return nil
}

// trigramSet builds the set of normalized trigrams for a piece of reply
// content. We do char-level trigrams (not word-level) so the comparison
// stays reasonable for reply-sized prose without a tokenizer dependency.
//
// Normalization steps:
//   - lower case
//   - collapse all whitespace runs to a single space
//   - drop punctuation that LLMs love to vary (em dashes, smart quotes,
//     trailing exclamations) so a "rephrase" with different punctuation
//     does not slip past the dedup gate
//   - bracket the string with two spaces on each end so the first and
//     last trigrams capture the prefix/suffix shape
//
// Texts shorter than ~24 characters trigger an empty set; the caller
// short-circuits in that case (a 5-char "+1" reply has no business being
// dedup-rejected).
func trigramSet(text string) map[string]struct{} {
	cleaned := normalizeForTrigrams(text)
	if len([]rune(cleaned)) < 24 {
		return nil
	}
	cleaned = "  " + cleaned + "  "
	r := []rune(cleaned)
	out := make(map[string]struct{}, len(r))
	for i := 0; i+3 <= len(r); i++ {
		out[string(r[i:i+3])] = struct{}{}
	}
	return out
}

func normalizeForTrigrams(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		case r == '“' || r == '”' || r == '‘' || r == '’' || r == '"' || r == '\'':
			// drop quotes — agents sprinkle these inconsistently
		case r == '—' || r == '–' || r == '-':
			b.WriteByte(' ')
			prevSpace = true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
		default:
			// drop other punctuation: . , ! ? : ; etc.
		}
	}
	return strings.TrimSpace(b.String())
}

// jaccard returns the trigram-set Jaccard similarity in [0, 1]. Empty
// inputs return 0 so the caller treats them as "not similar enough to
// reject" — short replies get a free pass.
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	// Iterate the smaller set for cheaper union math.
	small, large := a, b
	if len(b) < len(a) {
		small, large = b, a
	}
	for k := range small {
		if _, ok := large[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
