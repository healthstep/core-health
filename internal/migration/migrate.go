package migration

import (
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/gorm"
)

func Run(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.HealthCriterion{},
		&model.LabTest{},
		&model.LabTestCriterion{},
		&model.UserCriterionEvent{},
		&model.Document{},
		&model.NumericRecommendationRule{},
		&model.BooleanRecommendationRule{},
		&model.UserCriterionState{},
		&model.NotificationLog{},
	)
}
