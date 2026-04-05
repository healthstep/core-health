package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"github.com/helthtech/core-health/internal/repository"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

type HealthService struct {
	repo  *repository.HealthRepository
	cache *AnalysisCache
	nc    *nats.Conn
	redis *redis.Client
}

func NewHealthService(repo *repository.HealthRepository, nc *nats.Conn, rdb *redis.Client) *HealthService {
	return &HealthService{
		repo:  repo,
		cache: NewAnalysisCache(),
		nc:    nc,
		redis: rdb,
	}
}

// StartCache begins the in-memory cache refresh loop.
func (s *HealthService) StartCache(ctx context.Context) {
	go s.cache.RunRefreshLoop(ctx, s.repo)
}

// ListAnalysis returns analyses filtered by user sex and blocking rules.
// userID is optional (pass uuid.Nil to skip blocking check).
func (s *HealthService) ListAnalysis(ctx context.Context, userID uuid.UUID, userSex string) ([]model.Analysis, error) {
	analyses := s.cache.GetAnalyses()
	if len(analyses) == 0 {
		// Cache not loaded yet — fall back to DB.
		var err error
		analyses, err = s.repo.ListAnalysis(ctx)
		if err != nil {
			return nil, err
		}
	}

	allCriteria := s.cache.GetCriteria()

	// Build user values map for blocking check.
	userValues := map[uuid.UUID]string{}
	if userID != uuid.Nil {
		ucs, err := s.repo.GetUserCriteria(ctx, userID)
		if err == nil {
			for _, uc := range ucs {
				userValues[uc.CriterionID] = uc.Value
			}
		}
	}

	var result []model.Analysis
	for _, a := range analyses {
		if !MatchesSex(a, userSex) {
			continue
		}
		if userID != uuid.Nil && IsAnalysisBlocked(a, allCriteria, userValues) {
			continue
		}
		result = append(result, a)
	}
	return result, nil
}

func (s *HealthService) ListCriteria(ctx context.Context, analysisID string) ([]model.Criterion, error) {
	if analysisID == "" {
		return s.repo.ListCriteria(ctx, nil)
	}
	id, err := uuid.Parse(analysisID)
	if err != nil {
		return nil, fmt.Errorf("invalid analysis_id: %w", err)
	}
	return s.repo.ListCriteria(ctx, &id)
}

// SetUserCriterion stores or updates a user's criterion value.
func (s *HealthService) SetUserCriterion(ctx context.Context, userID, criterionID uuid.UUID, value, source string) error {
	uc := &model.UserCriterion{
		UserID:      userID,
		CriterionID: criterionID,
		Value:       value,
		UpdatedAt:   time.Now(),
	}
	return s.repo.SetUserCriterion(ctx, uc)
}

// ResetAnalysisCriteria soft-deletes all user criteria for an analysis.
func (s *HealthService) ResetAnalysisCriteria(ctx context.Context, userID, analysisID uuid.UUID) error {
	return s.repo.SoftDeleteAnalysisCriteria(ctx, userID, analysisID)
}

// GetUserCriteria returns enriched entries: all criteria with user values + recommendations.
// Sex is used to skip analyses that don't match the user's sex.
func (s *HealthService) GetUserCriteria(ctx context.Context, userID uuid.UUID, userSex string) ([]UserCriterionEntry, error) {
	criteria := s.cache.GetCriteria()
	if len(criteria) == 0 {
		var err error
		criteria, err = s.repo.ListCriteria(ctx, nil)
		if err != nil {
			return nil, err
		}
	}

	analyses := s.cache.GetAnalyses()
	if len(analyses) == 0 {
		var err error
		analyses, err = s.repo.ListAnalysis(ctx)
		if err != nil {
			return nil, err
		}
	}

	analysisMap := make(map[uuid.UUID]model.Analysis, len(analyses))
	for _, a := range analyses {
		analysisMap[a.ID] = a
	}

	userCriteria, err := s.repo.GetUserCriteria(ctx, userID)
	if err != nil {
		return nil, err
	}
	valueMap := make(map[uuid.UUID]string, len(userCriteria))
	for _, uc := range userCriteria {
		valueMap[uc.CriterionID] = uc.Value
	}

	entries := make([]UserCriterionEntry, 0, len(criteria))
	for _, c := range criteria {
		analysis := analysisMap[c.AnalysisID]

		// Skip criteria belonging to sex-restricted analyses.
		if !MatchesSex(analysis, userSex) {
			continue
		}

		value := valueMap[c.ID]
		rules := s.cache.GetRulesForCriterion(c.ID)
		rule := repository.EvaluateCriterionStatus(value, rules)

		entry := UserCriterionEntry{
			CriterionID:   c.ID.String(),
			CriterionName: c.Name,
			AnalysisID:    c.AnalysisID.String(),
			AnalysisName:  analysis.Name,
			Value:         value,
			Level:         c.Level,
		}
		if rule != nil {
			entry.Recommendation = rule.Recommendation
			entry.Status = rule.Severity
			entry.Severity = rule.Severity
		} else if value == "" {
			entry.Status = "empty"
		} else {
			entry.Status = "ok"
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetProgress computes fill statistics for a user.
func (s *HealthService) GetProgress(ctx context.Context, userID uuid.UUID) (*ProgressResult, error) {
	criteria, err := s.repo.ListCriteria(ctx, nil)
	if err != nil {
		return nil, err
	}
	userCriteria, err := s.repo.GetUserCriteria(ctx, userID)
	if err != nil {
		return nil, err
	}

	filled := 0
	for _, uc := range userCriteria {
		if uc.Value != "" {
			filled++
		}
	}
	total := len(criteria)
	pct := 0.0
	if total > 0 {
		pct = float64(filled) / float64(total) * 100
	}

	return &ProgressResult{
		Total:      total,
		Filled:     filled,
		Percent:    pct,
		LevelLabel: computeLevelLabel(pct),
	}, nil
}

// --- Weighted auction ---

// recommendationWeight returns base weight for a severity.
func recommendationWeight(severity string) int {
	switch severity {
	case "critical":
		return 10
	case "empty":
		return 6
	case "warning":
		return 4
	default:
		return 1
	}
}

// weeklyKey returns the Redis key for tracking weekly sent recommendations.
func weeklyKey(userID uuid.UUID, criterionID string) string {
	return fmt.Sprintf("weekly_sent:%s:%s", userID.String(), criterionID)
}

// GetRecommendations returns ranked recommendations using a weighted auction.
func (s *HealthService) GetRecommendations(ctx context.Context, userID uuid.UUID, userSex string) ([]RecommendationItem, error) {
	entries, err := s.GetUserCriteria(ctx, userID, userSex)
	if err != nil {
		return nil, err
	}

	type candidate struct {
		item   RecommendationItem
		weight int
	}

	var candidates []candidate

	for _, e := range entries {
		if e.Status == "ok" || e.Status == "" {
			continue
		}

		sev := e.Status
		if sev == "" {
			sev = "empty"
		}

		baseWeight := recommendationWeight(sev)

		// Penalise if sent this week.
		key := weeklyKey(userID, e.CriterionID)
		sentThisWeek, _ := s.redis.Exists(ctx, key).Result()
		if sentThisWeek > 0 {
			baseWeight -= 4
		}
		if baseWeight < 1 {
			baseWeight = 1
		}

		candidates = append(candidates, candidate{
			item: RecommendationItem{
				CriterionID:   e.CriterionID,
				CriterionName: e.CriterionName,
				AnalysisName:  e.AnalysisName,
				Text:          e.Recommendation,
				Severity:      sev,
			},
			weight: baseWeight,
		})
	}

	// Sort by weight descending for display.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].weight > candidates[j].weight
	})

	result := make([]RecommendationItem, 0, len(candidates))
	for _, c := range candidates {
		result = append(result, c.item)
	}
	return result, nil
}

// SelectDailyRecommendation picks one recommendation using weighted random selection.
func (s *HealthService) SelectDailyRecommendation(ctx context.Context, userID uuid.UUID, userSex string) (*RecommendationItem, error) {
	recs, err := s.GetRecommendations(ctx, userID, userSex)
	if err != nil {
		return nil, err
	}

	if len(recs) == 0 {
		return &RecommendationItem{
			Text:     "🎉 Все показатели заполнены и в норме! Попробуйте добавить показатели следующего уровня.",
			Severity: "ok",
		}, nil
	}

	// Build weighted pool.
	type weightedItem struct {
		item   RecommendationItem
		weight int
	}
	var pool []weightedItem
	totalWeight := 0
	for _, r := range recs {
		w := recommendationWeight(r.Severity)
		key := weeklyKey(userID, r.CriterionID)
		if sent, _ := s.redis.Exists(ctx, key).Result(); sent > 0 {
			w -= 4
		}
		if w < 1 {
			w = 1
		}
		pool = append(pool, weightedItem{item: r, weight: w})
		totalWeight += w
	}

	// Weighted random pick.
	pick := rand.Intn(totalWeight)
	var chosen *RecommendationItem
	for _, wi := range pool {
		pick -= wi.weight
		if pick < 0 {
			item := wi.item
			chosen = &item
			break
		}
	}
	if chosen == nil {
		item := pool[0].item
		chosen = &item
	}

	// Store in Redis + mark as sent this week.
	recKey := "daily_rec:" + userID.String()
	data, _ := json.Marshal(chosen)
	s.redis.Set(ctx, recKey, string(data), 24*time.Hour)
	if chosen.CriterionID != "" {
		s.redis.Set(ctx, weeklyKey(userID, chosen.CriterionID), "1", 7*24*time.Hour)
	}

	return chosen, nil
}

// GetCachedDailyRecommendation retrieves the cached daily recommendation from Redis.
func (s *HealthService) GetCachedDailyRecommendation(ctx context.Context, userID uuid.UUID, userSex string) (*RecommendationItem, error) {
	recKey := "daily_rec:" + userID.String()
	data, err := s.redis.Get(ctx, recKey).Result()
	if err == redis.Nil {
		return s.SelectDailyRecommendation(ctx, userID, userSex)
	}
	if err != nil {
		return nil, err
	}
	var item RecommendationItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// SendNotification publishes a NATS notification message.
func (s *HealthService) SendNotification(ctx context.Context, userID uuid.UUID, channel, notifType, templateCode, payloadJSON string) error {
	logEntry := &model.NotificationLog{
		ID:               uuid.New(),
		UserID:           userID,
		Channel:          channel,
		NotificationType: notifType,
		TemplateCode:     templateCode,
		PayloadSummary:   payloadJSON,
		SentAt:           time.Now(),
		DeliveryStatus:   "sent",
	}
	_ = s.repo.CreateNotificationLog(ctx, logEntry)

	msg := NotificationMessage{
		UserID:           userID.String(),
		NotificationType: notifType,
		TemplateCode:     templateCode,
		PayloadJSON:      payloadJSON,
	}
	d, _ := json.Marshal(msg)
	subject := "notification." + strings.ToLower(channel)
	return s.nc.Publish(subject, d)
}

// RunDailyScheduler fires at 12:00 and 20:00 — standard recommendation notifications.
func (s *HealthService) RunDailyScheduler(ctx context.Context, channels []string) {
	for {
		next := nextScheduledTime([]int{12, 20})
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}

		userIDs, err := s.repo.GetAllDistinctUserIDs(ctx)
		if err != nil {
			continue
		}

		for _, userID := range userIDs {
			rec, err := s.SelectDailyRecommendation(ctx, userID, "")
			if err != nil {
				continue
			}
			payload, _ := json.Marshal(map[string]string{
				"title": "Рекомендация дня",
				"body":  rec.Text,
			})
			for _, ch := range channels {
				_ = s.SendNotification(ctx, userID, ch, "daily_recommendation", "daily_rec", string(payload))
			}
		}
	}
}

// RunExpiryScheduler fires at 9:00 — sends expiry reminders and cleans up expired data.
// Reminders are sent at most once every 3 days per (user, analysis) pair.
func (s *HealthService) RunExpiryScheduler(ctx context.Context, channels []string) {
	for {
		next := nextScheduledTime([]int{9})
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}

		// Clean up expired criteria first.
		if err := s.repo.SoftDeleteExpiredCriteria(ctx); err != nil {
			continue
		}

		// Warn about near-expiry (within 30 days).
		entries, err := s.repo.GetNearExpiryEntries(ctx, 30*24*time.Hour)
		if err != nil {
			continue
		}

		for _, e := range entries {
			dedupeKey := fmt.Sprintf("expiry_notif:%s:%s", e.UserID.String(), e.Analysis.ID.String())
			// Send at most once every 3 days.
			if exists, _ := s.redis.Exists(ctx, dedupeKey).Result(); exists > 0 {
				continue
			}

			daysLeft := int(time.Until(e.ExpiresAt).Hours() / 24)
			payload, _ := json.Marshal(map[string]string{
				"title":    "Напоминание: повторите анализ",
				"body":     fmt.Sprintf("Срок действия анализа «%s» истекает через %d дн. Пройдите его снова.", e.Analysis.Name, daysLeft),
				"analysis": e.Analysis.Name,
			})

			for _, ch := range channels {
				_ = s.SendNotification(ctx, e.UserID, ch, "expiry_reminder", "expiry_reminder", string(payload))
			}

			// Set 3-day dedup key.
			s.redis.Set(ctx, dedupeKey, "1", 3*24*time.Hour)
		}
	}
}

// nextScheduledTime returns the next upcoming occurrence of one of the given hours (local time).
func nextScheduledTime(hours []int) time.Time {
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	var candidates []time.Time
	for _, h := range hours {
		candidates = append(candidates, today.Add(time.Duration(h)*time.Hour))
		candidates = append(candidates, today.Add(24*time.Hour+time.Duration(h)*time.Hour))
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Before(candidates[j]) })
	for _, t := range candidates {
		if t.After(now) {
			return t
		}
	}
	return candidates[len(candidates)-1]
}

// EvaluateCriterionValue returns a numeric value from string, or 0 and false if not numeric.
func EvaluateCriterionValue(value string) (float64, bool) {
	if value == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(value, 64)
	return f, err == nil
}

func computeLevelLabel(pct float64) string {
	switch {
	case pct >= 80:
		return "Под полным контролем"
	case pct >= 50:
		return "Хорошая осведомлённость"
	default:
		return "Начало пути"
	}
}

// --- Value types ---

type NotificationMessage struct {
	UserID           string `json:"user_id"`
	NotificationType string `json:"notification_type"`
	TemplateCode     string `json:"template_code"`
	PayloadJSON      string `json:"payload_json"`
}

type UserCriterionEntry struct {
	CriterionID    string
	CriterionName  string
	AnalysisID     string
	AnalysisName   string
	Value          string
	Status         string
	Recommendation string
	Severity       string
	Level          int
}

type ProgressResult struct {
	Total      int
	Filled     int
	Percent    float64
	LevelLabel string
}

type RecommendationItem struct {
	CriterionID   string `json:"criterion_id"`
	CriterionName string `json:"criterion_name"`
	AnalysisName  string `json:"analysis_name"`
	Text          string `json:"text"`
	Severity      string `json:"severity"`
}
