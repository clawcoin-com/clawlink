package handlers

import (
    "time"

    "github.com/clawcoin-com/clawlink/internal/core/models"
    "gorm.io/gorm"
)

// recalcTagCounters recomputes post_count and last_used_at for the provided
// tag IDs from source-of-truth tables (posts + post_tags), instead of trying
// to maintain fragile +1/-1 deltas in every code path.
//
// Rules:
//   - post_count = COUNT(post_tags rows for tag_id)
//   - last_used_at = MAX(posts.created_at for tagged posts)
//   - if a tag has zero posts left, post_count=0 and last_used_at keeps its
//     previous value if non-zero, else falls back to now (so sorting remains stable)
//
// This helper is safe to call repeatedly inside a transaction and is the only
// place that should mutate these cached fields.
func recalcTagCounters(tx *gorm.DB, tagIDs ...string) error {
    for _, tagID := range tagIDs {
        if tagID == "" {
            continue
        }

        var postCount int64
        if err := tx.Model(&models.PostTag{}).Where("tag_id = ?", tagID).Count(&postCount).Error; err != nil {
            return err
        }

        // Find the most recent post creation time among posts using this tag.
        var latest struct {
            CreatedAt *time.Time `gorm:"column:created_at"`
        }
        if err := tx.Model(&models.Post{}).
            Select("MAX(posts.created_at) AS created_at").
            Joins("JOIN post_tags ON post_tags.post_id = posts.id").
            Where("post_tags.tag_id = ?", tagID).
            Scan(&latest).Error; err != nil {
            return err
        }

        updates := map[string]interface{}{
            "post_count": int(postCount),
            "updated_at": time.Now(),
        }
        if latest.CreatedAt != nil {
            updates["last_used_at"] = *latest.CreatedAt
        }

        if err := tx.Model(&models.Tag{}).Where("id = ?", tagID).Updates(updates).Error; err != nil {
            return err
        }
    }
    return nil
}
