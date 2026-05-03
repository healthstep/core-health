package migration

import (
	"strings"

	"github.com/helthtech/core-health/internal/model"
	"gorm.io/gorm"
)

func Run(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.CriterionGroup{},
		&model.Analysis{},
		&model.Criterion{},
		&model.UserCriterion{},
		&model.Recommendation{},
		&model.RecommendationNotification{},
		&model.WeeklyRecommendation{},
		&model.NotificationLog{},
	); err != nil {
		return err
	}
	if err := migrateCriterionInstructionToAnalyses(db); err != nil {
		return err
	}
	return migrateLegacyRecommendationTexts(db)
}

// migrateCriterionInstructionToAnalyses moves legacy criteria.instruction into analyses rows
// and drops the instruction column once done.
func migrateCriterionInstructionToAnalyses(db *gorm.DB) error {
	var hasInstr bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = CURRENT_SCHEMA()
			  AND table_name = 'criteria'
			  AND column_name = 'instruction'
		)`).Scan(&hasInstr).Error; err != nil {
		return err
	}
	if !hasInstr {
		return nil
	}

	type row struct {
		ID          string
		Name        string
		Instruction string
	}
	var rows []row
	if err := db.Raw(`
		SELECT id::text, name, COALESCE(instruction, '') AS instruction
		FROM criteria
		WHERE TRIM(COALESCE(instruction, '')) != ''
	`).Scan(&rows).Error; err != nil {
		return err
	}

	for _, r := range rows {
		a := model.Analysis{
			Name:        r.Name,
			Instruction: strings.TrimSpace(r.Instruction),
		}
		if err := db.Create(&a).Error; err != nil {
			return err
		}
		if err := db.Exec(`UPDATE criteria SET analysis_id = ? WHERE id = ?::uuid`, a.ID, r.ID).Error; err != nil {
			return err
		}
	}

	return db.Exec(`ALTER TABLE criteria DROP COLUMN IF EXISTS instruction`).Error
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
