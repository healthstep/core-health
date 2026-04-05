package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HealthRepository struct {
	db *gorm.DB
}

func NewHealthRepository(db *gorm.DB) *HealthRepository {
	return &HealthRepository{db: db}
}

func (r *HealthRepository) ListAnalysis(ctx context.Context) ([]model.Analysis, error) {
	var analyses []model.Analysis
	err := r.db.WithContext(ctx).Order("name").Find(&analyses).Error
	return analyses, err
}

func (r *HealthRepository) ListCriteria(ctx context.Context, analysisID *uuid.UUID) ([]model.Criterion, error) {
	var criteria []model.Criterion
	q := r.db.WithContext(ctx).Order("level, name")
	if analysisID != nil {
		q = q.Where("analysis_id = ?", *analysisID)
	}
	err := q.Find(&criteria).Error
	return criteria, err
}

func (r *HealthRepository) GetCriterion(ctx context.Context, id uuid.UUID) (*model.Criterion, error) {
	var c model.Criterion
	err := r.db.WithContext(ctx).First(&c, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SetUserCriterion upserts a user_criterion record (insert or update on conflict).
// Also restores soft-deleted records.
func (r *HealthRepository) SetUserCriterion(ctx context.Context, uc *model.UserCriterion) error {
	// Restore if soft-deleted, or upsert normally.
	return r.db.WithContext(ctx).
		Unscoped().
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "criterion_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"value":      uc.Value,
				"updated_at": uc.UpdatedAt,
				"deleted_at": nil,
			}),
		}).
		Create(uc).Error
}

func (r *HealthRepository) GetUserCriteria(ctx context.Context, userID uuid.UUID) ([]model.UserCriterion, error) {
	var ucs []model.UserCriterion
	// GORM automatically excludes soft-deleted records.
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&ucs).Error
	return ucs, err
}

// SoftDeleteAnalysisCriteria soft-deletes all user criteria for an analysis.
func (r *HealthRepository) SoftDeleteAnalysisCriteria(ctx context.Context, userID, analysisID uuid.UUID) error {
	// Get all criterion IDs for this analysis.
	var criterionIDs []uuid.UUID
	err := r.db.WithContext(ctx).
		Model(&model.Criterion{}).
		Where("analysis_id = ?", analysisID).
		Pluck("id", &criterionIDs).Error
	if err != nil || len(criterionIDs) == 0 {
		return err
	}

	return r.db.WithContext(ctx).
		Where("user_id = ? AND criterion_id IN ?", userID, criterionIDs).
		Delete(&model.UserCriterion{}).Error
}

// SoftDeleteExpiredCriteria finds and soft-deletes criteria that have exceeded their analysis lifetime.
func (r *HealthRepository) SoftDeleteExpiredCriteria(ctx context.Context) error {
	var analyses []model.Analysis
	if err := r.db.WithContext(ctx).Where("lifetime > 0").Find(&analyses).Error; err != nil {
		return err
	}

	now := time.Now()
	for _, a := range analyses {
		expiryCutoff := now.Add(-time.Duration(a.Lifetime) * 24 * time.Hour)

		var criterionIDs []uuid.UUID
		r.db.WithContext(ctx).
			Model(&model.Criterion{}).
			Where("analysis_id = ?", a.ID).
			Pluck("id", &criterionIDs)

		if len(criterionIDs) == 0 {
			continue
		}

		// Soft-delete criteria updated before the expiry cutoff.
		r.db.WithContext(ctx).
			Where("criterion_id IN ? AND updated_at < ?", criterionIDs, expiryCutoff).
			Delete(&model.UserCriterion{})
	}
	return nil
}

// GetUsersWithNearExpiryAnalyses returns (userID, analysis) pairs where the user's
// data is within warnWithin of expiring.
type NearExpiryEntry struct {
	UserID     uuid.UUID
	Analysis   model.Analysis
	ExpiresAt  time.Time
}

func (r *HealthRepository) GetNearExpiryEntries(ctx context.Context, warnWithin time.Duration) ([]NearExpiryEntry, error) {
	var analyses []model.Analysis
	if err := r.db.WithContext(ctx).Where("lifetime > 0").Find(&analyses).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	var result []NearExpiryEntry

	for _, a := range analyses {
		lifetime := time.Duration(a.Lifetime) * 24 * time.Hour

		var criterionIDs []uuid.UUID
		r.db.WithContext(ctx).
			Model(&model.Criterion{}).
			Where("analysis_id = ?", a.ID).
			Pluck("id", &criterionIDs)
		if len(criterionIDs) == 0 {
			continue
		}

		// Find the latest updated_at per user for this analysis.
		type row struct {
			UserID    uuid.UUID
			UpdatedAt time.Time
		}
		var rows []row
		r.db.WithContext(ctx).
			Model(&model.UserCriterion{}).
			Select("user_id, MAX(updated_at) as updated_at").
			Where("criterion_id IN ?", criterionIDs).
			Group("user_id").
			Scan(&rows)

		for _, rw := range rows {
			expiresAt := rw.UpdatedAt.Add(lifetime)
			timeLeft := expiresAt.Sub(now)
			if timeLeft > 0 && timeLeft <= warnWithin {
				result = append(result, NearExpiryEntry{
					UserID:    rw.UserID,
					Analysis:  a,
					ExpiresAt: expiresAt,
				})
			}
		}
	}
	return result, nil
}

func (r *HealthRepository) GetRecommendationRules(ctx context.Context, criterionID uuid.UUID) ([]model.RecommendationRule, error) {
	var rules []model.RecommendationRule
	err := r.db.WithContext(ctx).Where("criterion_id = ?", criterionID).Find(&rules).Error
	return rules, err
}

func (r *HealthRepository) GetAllRecommendationRules(ctx context.Context) ([]model.RecommendationRule, error) {
	var rules []model.RecommendationRule
	err := r.db.WithContext(ctx).Find(&rules).Error
	return rules, err
}

func (r *HealthRepository) CreateNotificationLog(ctx context.Context, n *model.NotificationLog) error {
	return r.db.WithContext(ctx).Create(n).Error
}

// GetAllDistinctUserIDs returns all user IDs that have at least one criterion entry.
func (r *HealthRepository) GetAllDistinctUserIDs(ctx context.Context) ([]uuid.UUID, error) {
	var rows []struct{ UserID uuid.UUID }
	err := r.db.WithContext(ctx).
		Model(&model.UserCriterion{}).
		Select("DISTINCT user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	return ids, nil
}

// EvaluateCriterionStatus finds the matching recommendation rule for a value.
// Sex filtering is done at the Analysis level; no sex param here.
func EvaluateCriterionStatus(value string, rules []model.RecommendationRule) *model.RecommendationRule {
	if value == "" {
		for i := range rules {
			if rules[i].MinValue == nil && rules[i].MaxValue == nil {
				return &rules[i]
			}
		}
		return nil
	}

	numVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		// Non-numeric (mark-done style): return the first "ok" rule.
		for i := range rules {
			if rules[i].Severity == "ok" {
				return &rules[i]
			}
		}
		return nil
	}

	for i := range rules {
		r := &rules[i]
		if r.MinValue == nil && r.MaxValue == nil {
			continue
		}
		if r.MinValue != nil && numVal < *r.MinValue {
			continue
		}
		if r.MaxValue != nil && numVal >= *r.MaxValue {
			continue
		}
		return r
	}
	return nil
}
