package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HealthRepository struct {
	db *gorm.DB
}

func NewHealthRepository(db *gorm.DB) *HealthRepository {
	return &HealthRepository{db: db}
}

// --- Groups ---

func (r *HealthRepository) ListGroups(ctx context.Context) ([]model.CriterionGroup, error) {
	var groups []model.CriterionGroup
	err := r.db.WithContext(ctx).Order("sort_order, name").Find(&groups).Error
	return groups, err
}

func (r *HealthRepository) UpsertGroup(ctx context.Context, g *model.CriterionGroup) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "sort_order"}),
	}).Create(g).Error
}

// --- Criteria ---

func (r *HealthRepository) ListCriteria(ctx context.Context) ([]model.Criterion, error) {
	var criteria []model.Criterion
	err := r.db.WithContext(ctx).Order("level, sort_order, name").Find(&criteria).Error
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

func (r *HealthRepository) UpsertCriterion(ctx context.Context, c *model.Criterion) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"group_id", "name", "level", "sex", "blocked_by", "input_type", "lifetime", "sort_order"}),
	}).Create(c).Error
}

// --- User Criteria ---

// SetUserCriterion upserts a user_criterion record (insert or update on conflict).
// Also restores soft-deleted records.
func (r *HealthRepository) SetUserCriterion(ctx context.Context, uc *model.UserCriterion) error {
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
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&ucs).Error
	return ucs, err
}

// SoftDeleteAllUserCriteria soft-deletes all user criteria for a user.
func (r *HealthRepository) SoftDeleteAllUserCriteria(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.UserCriterion{}).Error
}

// SoftDeleteExpiredCriteria finds and soft-deletes user_criteria that have exceeded the
// criterion's lifetime.
func (r *HealthRepository) SoftDeleteExpiredCriteria(ctx context.Context) error {
	var criteria []model.Criterion
	if err := r.db.WithContext(ctx).Where("lifetime > 0").Find(&criteria).Error; err != nil {
		return err
	}

	now := time.Now()
	for _, c := range criteria {
		expiryCutoff := now.Add(-time.Duration(c.Lifetime) * 24 * time.Hour)
		r.db.WithContext(ctx).
			Where("criterion_id = ? AND updated_at < ?", c.ID, expiryCutoff).
			Delete(&model.UserCriterion{})
	}
	return nil
}

// NearExpiryEntry holds a user + criterion that is nearing expiry.
type NearExpiryEntry struct {
	UserID    uuid.UUID
	Criterion model.Criterion
	ExpiresAt time.Time
}

// GetNearExpiryEntries returns (userID, criterion) pairs where the user's data is
// within warnWithin of expiring.
func (r *HealthRepository) GetNearExpiryEntries(ctx context.Context, warnWithin time.Duration) ([]NearExpiryEntry, error) {
	var criteria []model.Criterion
	if err := r.db.WithContext(ctx).Where("lifetime > 0").Find(&criteria).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	var result []NearExpiryEntry

	for _, c := range criteria {
		lifetime := time.Duration(c.Lifetime) * 24 * time.Hour

		type row struct {
			UserID    uuid.UUID
			UpdatedAt time.Time
		}
		var rows []row
		r.db.WithContext(ctx).
			Model(&model.UserCriterion{}).
			Select("user_id, updated_at").
			Where("criterion_id = ?", c.ID).
			Scan(&rows)

		for _, rw := range rows {
			expiresAt := rw.UpdatedAt.Add(lifetime)
			timeLeft := expiresAt.Sub(now)
			if timeLeft > 0 && timeLeft <= warnWithin {
				result = append(result, NearExpiryEntry{
					UserID:    rw.UserID,
					Criterion: c,
					ExpiresAt: expiresAt,
				})
			}
		}
	}
	return result, nil
}

// --- Recommendation Rules ---

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

// --- Recommendations (notification/auction system) ---

func (r *HealthRepository) GetAllRecommendations(ctx context.Context) ([]model.Recommendation, error) {
	var recs []model.Recommendation
	err := r.db.WithContext(ctx).Find(&recs).Error
	return recs, err
}

func (r *HealthRepository) GetRecommendationsByCriterion(ctx context.Context, criterionID uuid.UUID) ([]model.Recommendation, error) {
	var recs []model.Recommendation
	err := r.db.WithContext(ctx).Where("criterion_id = ?", criterionID).Find(&recs).Error
	return recs, err
}

func (r *HealthRepository) UpsertRecommendation(ctx context.Context, rec *model.Recommendation) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"criterion_id", "type", "title", "texts", "base_weight", "min_value", "max_value"}),
	}).Create(rec).Error
}

func (r *HealthRepository) DeleteRecommendation(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Recommendation{}, "id = ?", id).Error
}

// --- Weekly Recommendations ---

func (r *HealthRepository) GetWeeklyRecommendation(ctx context.Context, userID uuid.UUID, weekStart time.Time) (*model.WeeklyRecommendation, error) {
	var wr model.WeeklyRecommendation
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND week_start = ?", userID, weekStart).
		First(&wr).Error
	if err != nil {
		return nil, err
	}
	return &wr, nil
}

func (r *HealthRepository) UpsertWeeklyRecommendation(ctx context.Context, wr *model.WeeklyRecommendation) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "week_start"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"weights":    wr.Weights,
			"updated_at": wr.UpdatedAt,
		}),
	}).Create(wr).Error
}

func (r *HealthRepository) SaveWeeklyWeights(ctx context.Context, userID uuid.UUID, weekStart time.Time, weights map[string]int) error {
	wr := &model.WeeklyRecommendation{
		UserID:    userID,
		WeekStart: weekStart,
		Weights:   datatypes.NewJSONType(weights),
		UpdatedAt: time.Now(),
	}
	return r.UpsertWeeklyRecommendation(ctx, wr)
}

// --- Notifications ---

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
		// Non-numeric (check/boolean-type): return the first "ok" rule.
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
