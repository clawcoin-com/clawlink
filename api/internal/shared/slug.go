package shared

import (
    "crypto/sha1"
    "encoding/hex"
    "regexp"
    "strings"
)

var nonSlugChar = regexp.MustCompile(`[^a-z0-9-]+`)
var multiDash = regexp.MustCompile(`-+`)

// MakeSlug converts an arbitrary tag name into a stable ASCII slug.
//
// Strategy:
//  1. lowercase + trim spaces
//  2. keep ASCII letters/digits
//  3. spaces / punctuation → '-'
//  4. collapse duplicate '-'
//  5. if result is empty (e.g. pure Chinese), fallback to a short hash
//  6. max 48 chars
//
// This is intentionally conservative: display uses Tag.Name (unicode-safe),
// URL uses Tag.Slug (ASCII-only, stable, shareable).
func MakeSlug(name string) string {
    raw := strings.TrimSpace(strings.ToLower(name))
    raw = strings.ReplaceAll(raw, "_", "-")
    raw = strings.ReplaceAll(raw, " ", "-")
    slug := nonSlugChar.ReplaceAllString(raw, "-")
    slug = multiDash.ReplaceAllString(slug, "-")
    slug = strings.Trim(slug, "-")
    if slug == "" {
        sum := sha1.Sum([]byte(name))
        slug = "tag-" + hex.EncodeToString(sum[:])[:10]
    }
    if len(slug) > 48 {
        slug = strings.Trim(slug[:48], "-")
        if slug == "" {
            sum := sha1.Sum([]byte(name))
            slug = "tag-" + hex.EncodeToString(sum[:])[:10]
        }
    }
    return slug
}
