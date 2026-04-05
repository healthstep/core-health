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
	cache *CriteriaCache
	nc    *nats.Conn
	redis *redis.Client
}

func NewHealthService(repo *repository.HealthRepository, nc *nats.Conn, rdb *redis.Client) *HealthService {
	return &HealthService{
		repo:  repo,
		cache: NewCriteriaCache(),
		nc:    nc,
		redis: rdb,
	}
}

// StartCache begins the in-memory cache refresh loop.
func (s *HealthService) StartCache(ctx context.Context) {
	go s.cache.RunRefreshLoop(ctx, s.repo)
}

// ListCriteria returns criteria filtered by user sex and blocking rules.
// userID is optional (pass uuid.Nil to skip blocking check).
func (s *HealthService) ListCriteria(ctx context.Context, userID uuid.UUID, userSex string) ([]model.Criterion, error) {
	allCriteria := s.cache.GetCriteria()
	if len(allCriteria) == 0 {
		var err error
		allCriteria, err = s.repo.ListCriteria(ctx)
		if err != nil {
			return nil, err
		}
	}

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

	var result []model.Criterion
	for _, c := range allCriteria {
		if !CriterionMatchesSex(c, userSex) {
			continue
		}
		if userID != uuid.Nil && IsCriterionBlocked(c, allCriteria, userValues) {
			continue
		}
		result = append(result, c)
	}
	return result, nil
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

// ResetAllCriteria soft-deletes all user criteria.
func (s *HealthService) ResetAllCriteria(ctx context.Context, userID uuid.UUID) error {
	return s.repo.SoftDeleteAllUserCriteria(ctx, userID)
}

// GetUserCriteria returns enriched entries: all visible criteria with user values + recommendations.
func (s *HealthService) GetUserCriteria(ctx context.Context, userID uuid.UUID, userSex string) ([]UserCriterionEntry, error) {
	allCriteria := s.cache.GetCriteria()
	if len(allCriteria) == 0 {
		var err error
		allCriteria, err = s.repo.ListCriteria(ctx)
		if err != nil {
			return nil, err
		}
	}

	userCriteria, err := s.repo.GetUserCriteria(ctx, userID)
	if err != nil {
		return nil, err
	}
	valueMap := make(map[uuid.UUID]string, len(userCriteria))
	for _, uc := range userCriteria {
		valueMap[uc.CriterionID] = uc.Value
	}

	// Build user values for blocking check.
	userValues := make(map[uuid.UUID]string, len(valueMap))
	for k, v := range valueMap {
		userValues[k] = v
	}

	entries := make([]UserCriterionEntry, 0, len(allCriteria))
	for _, c := range allCriteria {
		if !CriterionMatchesSex(c, userSex) {
			continue
		}

		value := valueMap[c.ID]
		rules := s.cache.GetRulesForCriterion(c.ID)
		rule := repository.EvaluateCriterionStatus(value, rules)

		entry := UserCriterionEntry{
			CriterionID:   c.ID.String(),
			CriterionName: c.Name,
			Value:         value,
			Level:         c.Level,
			InputType:     c.InputType,
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
	criteria, err := s.repo.ListCriteria(ctx)
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
				Text:          e.Recommendation,
				Severity:      sev,
			},
			weight: baseWeight,
		})
	}

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
func (s *HealthService) RunExpiryScheduler(ctx context.Context, channels []string) {
	for {
		next := nextScheduledTime([]int{9})
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}

		if err := s.repo.SoftDeleteExpiredCriteria(ctx); err != nil {
			continue
		}

		entries, err := s.repo.GetNearExpiryEntries(ctx, 30*24*time.Hour)
		if err != nil {
			continue
		}

		for _, e := range entries {
			dedupeKey := fmt.Sprintf("expiry_notif:%s:%s", e.UserID.String(), e.Criterion.ID.String())
			if exists, _ := s.redis.Exists(ctx, dedupeKey).Result(); exists > 0 {
				continue
			}

			daysLeft := int(time.Until(e.ExpiresAt).Hours() / 24)
			payload, _ := json.Marshal(map[string]string{
				"title":     "Напоминание: обновите показатель",
				"body":      fmt.Sprintf("Данные «%s» устареют через %d дн. Обновите их.", e.Criterion.Name, daysLeft),
				"criterion": e.Criterion.Name,
			})

			for _, ch := range channels {
				_ = s.SendNotification(ctx, e.UserID, ch, "expiry_reminder", "expiry_reminder", string(payload))
			}

			s.redis.Set(ctx, dedupeKey, "1", 3*24*time.Hour)
		}
	}
}

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
	Value          string
	Status         string
	Recommendation string
	Severity       string
	Level          int
	InputType      string
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
	Text          string `json:"text"`
	Severity      string `json:"severity"`
}
