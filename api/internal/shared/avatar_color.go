package shared

import (
	"crypto/sha1"
	"fmt"
)

// AvatarColorPalette is a small set of pleasant background colors used as
// the deterministic default for users without an uploaded avatar URL. The
// fixed palette guarantees we never produce muddy / low-contrast colors.
var AvatarColorPalette = []string{
	"#ef4444", // red
	"#f97316", // orange
	"#eab308", // amber
	"#22c55e", // green
	"#10b981", // emerald
	"#14b8a6", // teal
	"#06b6d4", // cyan
	"#3b82f6", // blue
	"#6366f1", // indigo
	"#8b5cf6", // violet
	"#a855f7", // purple
	"#ec4899", // pink
	"#f43f5e", // rose
}

// PickAvatarColor returns a stable color for the given seed (typically the
// user ID). Same seed → same color across calls.
func PickAvatarColor(seed string) string {
	if seed == "" {
		return AvatarColorPalette[0]
	}
	sum := sha1.Sum([]byte(seed))
	idx := int(sum[0]) % len(AvatarColorPalette)
	return AvatarColorPalette[idx]
}

// IsValidHexColor reports whether s is a 7-char lowercase hex like "#abcdef".
// Used by PUT handlers to validate user-supplied colors.
func IsValidHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// NormalizeHexColor lowercases a hex color. Caller should have validated
// beforehand. Used to keep DB rows consistent.
func NormalizeHexColor(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'F' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

// AvatarColorFor returns the color stored on a user, or generates a stable
// default from the ID. Always returns a valid hex string.
func AvatarColorFor(id, current string) string {
	if IsValidHexColor(current) {
		return NormalizeHexColor(current)
	}
	return PickAvatarColor(id)
}

// _ keeps fmt referenced for future debug logs without unused-import noise
// when the helpers are extended.
var _ = fmt.Sprintf
