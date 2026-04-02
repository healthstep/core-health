package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/gorm"
)

type HealthRepository struct {
	db *gorm.DB
}

func NewHealthRepository(db *gorm.DB) *HealthRepository {
	return &HealthRepository{db: db}
}

func (r *HealthRepository) ListCriteria(ctx context.Context) ([]model.HealthCriterion, error) {
	var criteria []model.HealthCriterion
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&criteria).Error
	return criteria, err
}

func (r *HealthRepository) ListLabTests(ctx context.Context) ([]model.LabTest, error) {
	var tests []model.LabTest
	err := r.db.WithContext(ctx).Find(&tests).Error
	return tests, err
}

func (r *HealthRepository) GetLabTestCriteria(ctx context.Context, labTestID uuid.UUID) ([]model.LabTestCriterion, error) {
	var links []model.LabTestCriterion
	err := r.db.WithContext(ctx).Where("lab_test_id = ?", labTestID).Find(&links).Error
	return links, err
}

func (r *HealthRepository) CreateEvent(ctx context.Context, e *model.UserCriterionEvent) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *HealthRepository) CreateDocument(ctx context.Context, d *model.Document) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *HealthRepository) GetUserEvents(ctx context.Context, userID uuid.UUID) ([]model.UserCriterionEvent, error) {
	var events []model.UserCriterionEvent
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("occurred_at DESC").Find(&events).Error
	return events, err
}

func (r *HealthRepository) GetUserCriterionStates(ctx context.Context, userID uuid.UUID) ([]model.UserCriterionState, error) {
	var states []model.UserCriterionState
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&states).Error
	return states, err
}

func (r *HealthRepository) UpsertCriterionState(ctx context.Context, s *model.UserCriterionState) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *HealthRepository) CreateNotificationLog(ctx context.Context, n *model.NotificationLog) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *HealthRepository) GetNumericRules(ctx context.Context, criterionID uuid.UUID) ([]model.NumericRecommendationRule, error) {
	var rules []model.NumericRecommendationRule
	err := r.db.WithContext(ctx).Where("health_criterion_id = ?", criterionID).Find(&rules).Error
	return rules, err
}

func (r *HealthRepository) GetBooleanRules(ctx context.Context, criterionID uuid.UUID) ([]model.BooleanRecommendationRule, error) {
	var rules []model.BooleanRecommendationRule
	err := r.db.WithContext(ctx).Where("health_criterion_id = ?", criterionID).Find(&rules).Error
	return rules, err
}
