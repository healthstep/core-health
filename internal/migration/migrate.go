package migration

import (
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/gorm"
)

func Run(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Analysis{},
		&model.Criterion{},
		&model.UserCriterion{},
		&model.RecommendationRule{},
		&model.NotificationLog{},
	)
}
