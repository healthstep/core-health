package migration

import (
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/gorm"
)

func Run(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.CriterionGroup{},
		&model.Criterion{},
		&model.UserCriterion{},
		&model.Recommendation{},
		&model.RecommendationNotification{},
		&model.WeeklyRecommendation{},
		&model.NotificationLog{},
	); err != nil {
		return err
	}
	return migrateLegacyRecommendationTexts(db)
}

// migrateLegacyRecommendationTexts copies legacy JSONB column recommendations.texts into
// recommendation_notifications, then drops the column (PostgreSQL).
func migrateLegacyRecommendationTexts(db *gorm.DB) error {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = CURRENT_SCHEMA()
			  AND table_name = 'recommendations'
			  AND column_name = 'texts'
		)`).Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if err := db.Exec(`
		INSERT INTO recommendation_notifications (id, recommendation_id, text, created_at)
		SELECT gen_random_uuid(), r.id, elem, NOW()
		FROM recommendations r
		CROSS JOIN LATERAL jsonb_array_elements_text(r.texts::jsonb) AS elem
		WHERE r.texts IS NOT NULL
		  AND jsonb_typeof(r.texts::jsonb) = 'array'
		  AND jsonb_array_length(r.texts::jsonb) > 0
		  AND NOT EXISTS (SELECT 1 FROM recommendation_notifications n WHERE n.recommendation_id = r.id)
	`).Error; err != nil {
		return err
	}
	return db.Exec(`ALTER TABLE recommendations DROP COLUMN IF EXISTS texts`).Error
}
