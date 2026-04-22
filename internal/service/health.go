package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
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

// ListGroups returns all criterion groups.
func (s *HealthService) ListGroups(ctx context.Context) ([]model.CriterionGroup, error) {
	groups := s.cache.GetGroups()
	if len(groups) == 0 {
		return s.repo.ListGroups(ctx)
	}
	return groups, nil
}

// ListCriteria returns criteria filtered by user sex.
func (s *HealthService) ListCriteria(ctx context.Context, userID uuid.UUID, userSex string) ([]model.Criterion, error) {
	allCriteria := s.cache.GetCriteria()
	if len(allCriteria) == 0 {
		var err error
		allCriteria, err = s.repo.ListCriteria(ctx)
		if err != nil {
			return nil, err
		}
	}

	var result []model.Criterion
	for _, c := range allCriteria {
		if !CriterionMatchesSex(c, userSex) {
			continue
		}
		result = append(result, c)
	}
	return result, nil
}

// SetUserCriterion stores or updates a user's criterion value.
func (s *HealthService) SetUserCriterion(ctx context.Context, userID, criterionID uuid.UUID, value, source, measuredAtStr string) error {
	uc := &model.UserCriterion{
		UserID:      userID,
		CriterionID: criterionID,
		Value:       value,
		UpdatedAt:   time.Now(),
	}
	if measuredAtStr != "" {
		if t, err := time.Parse("2006-01-02", measuredAtStr); err == nil {
			uc.MeasuredAt = &t
		}
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

	entries := make([]UserCriterionEntry, 0, len(allCriteria))
	for _, c := range allCriteria {
		if !CriterionMatchesSex(c, userSex) {
			continue
		}

		value := valueMap[c.ID]
		st, rec := DashboardCriterionStatus(c, value)

		groupID := ""
		if c.GroupID != nil {
			groupID = c.GroupID.String()
		}

		entry := UserCriterionEntry{
			CriterionID:      c.ID.String(),
			CriterionName:    c.Name,
			Value:            value,
			Level:            c.Level,
			InputType:        c.InputType,
			GroupID:          groupID,
			Status:           st,
			Severity:         st,
			Recommendation:   rec,
			Instruction:      c.Instruction,
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

// GetRecommendations returns ranked items derived from criterion status (for API / bots).
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

		baseWeight := ruleWeight(sev)

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

// --- Weekly recommendation system ---

// currentWeekStart returns the Monday of the current week (UTC, time truncated to midnight).
func currentWeekStart() time.Time {
	now := time.Now().UTC()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7 in ISO week
	}
	monday := now.AddDate(0, 0, -(weekday - 1))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

// isRecommendationApplicable checks if a Recommendation applies to the user given their current value.
//
// InputType semantics:
//   - numeric: MinValue/MaxValue define normal range; Delta defines non-critical deviation width
//   - check:   "1" = done (ok); "" = not done (reminder triggers)
//   - boolean: "1" = positive/ok; "0" = negative (alarm triggers); "" = no data (reminder)
//
// Recommendation types:
//   - reminder:             value == "" (no data entered)
//   - recommendation:       numeric only — value in warning (non-critical) zone
//   - alarm:                numeric with value outside warning zone, OR boolean with value "0"
//   - expiration_reminder:  handled by expiry scheduler (never selected here)
func isRecommendationApplicable(rec model.Recommendation, crit model.Criterion, value string) bool {
	switch rec.Type {
	case "reminder":
		return value == ""

	case "expiration_reminder":
		return false

	case "recommendation":
		if value == "" {
			return false
		}
		// recommendation only makes sense for numeric criteria with a defined normal range
		if crit.InputType != "numeric" {
			return false
		}
		if crit.MinValue == nil && crit.MaxValue == nil {
			return true // no range defined — always applicable
		}
		numVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		delta := 0.0
		if crit.Delta != nil {
			delta = *crit.Delta
		}
		inNormal := (crit.MinValue == nil || numVal >= *crit.MinValue) &&
			(crit.MaxValue == nil || numVal <= *crit.MaxValue)
		if inNormal {
			return false
		}
		// In non-critical (warning) zone?
		warnLow := math.Inf(-1)
		warnHigh := math.Inf(1)
		if crit.MinValue != nil {
			warnLow = *crit.MinValue - delta
		}
		if crit.MaxValue != nil {
			warnHigh = *crit.MaxValue + delta
		}
		return numVal >= warnLow && numVal <= warnHigh

	case "alarm":
		if value == "" {
			return false
		}
		// boolean: negative result ("0") is always an alarm
		if crit.InputType == "boolean" {
			return value == "0"
		}
		// numeric: value outside the warning zone
		if crit.InputType != "numeric" {
			return false
		}
		if crit.MinValue == nil && crit.MaxValue == nil {
			return false
		}
		numVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		delta := 0.0
		if crit.Delta != nil {
			delta = *crit.Delta
		}
		belowWarning := crit.MinValue != nil && numVal < *crit.MinValue-delta
		aboveWarning := crit.MaxValue != nil && numVal > *crit.MaxValue+delta
		return belowWarning || aboveWarning

	default:
		return false
	}
}

// GenerateWeeklyRecommendations builds the weekly recommendation weights for a user.
func (s *HealthService) GenerateWeeklyRecommendations(ctx context.Context, userID uuid.UUID, userSex string) (*WeeklyPlan, error) {
	weekStart := currentWeekStart()

	// Try to load existing weekly plan first.
	existing, err := s.repo.GetWeeklyRecommendation(ctx, userID, weekStart)
	if err == nil && existing != nil {
		weights := existing.Weights.Data()
		items := s.buildWeeklyItems(weights)
		return &WeeklyPlan{WeekStart: weekStart, Items: items, Weights: weights}, nil
	}

	// Build fresh plan.
	allCriteria := s.cache.GetCriteria()
	if len(allCriteria) == 0 {
		allCriteria, _ = s.repo.ListCriteria(ctx)
	}
	allRecs := s.cache.GetAllRecommendations()
	if len(allRecs) == 0 {
		allRecs, _ = s.repo.GetAllRecommendations(ctx)
	}

	// Get user values.
	userCriteria, _ := s.repo.GetUserCriteria(ctx, userID)
	valueMap := make(map[uuid.UUID]string)
	for _, uc := range userCriteria {
		valueMap[uc.CriterionID] = uc.Value
	}

	// Build criterion map for sex check.
	criterionMap := make(map[uuid.UUID]model.Criterion)
	for _, c := range allCriteria {
		criterionMap[c.ID] = c
	}

	weights := make(map[string]int)
	for _, rec := range allRecs {
		if rec.Type == "alarm" {
			continue // alarms go through separate scheduler
		}
		crit, ok := criterionMap[rec.CriterionID]
		if !ok {
			continue
		}
		if !CriterionMatchesSex(crit, userSex) {
			continue
		}
		value := valueMap[rec.CriterionID]
		if isRecommendationApplicable(rec, crit, value) {
			weights[rec.ID.String()] = rec.BaseWeight
		}
	}

	if err := s.repo.SaveWeeklyWeights(ctx, userID, weekStart, weights); err != nil {
		return nil, err
	}

	items := s.buildWeeklyItems(weights)
	return &WeeklyPlan{WeekStart: weekStart, Items: items, Weights: weights}, nil
}

func (s *HealthService) buildWeeklyItems(weights map[string]int) []WeeklyItem {
	allRecs := s.cache.GetAllRecommendations()
	recMap := make(map[string]model.Recommendation)
	for _, r := range allRecs {
		recMap[r.ID.String()] = r
	}
	allCriteria := s.cache.GetCriteria()
	criterionMap := make(map[uuid.UUID]model.Criterion)
	for _, c := range allCriteria {
		criterionMap[c.ID] = c
	}

	var items []WeeklyItem
	for recID, w := range weights {
		rec, ok := recMap[recID]
		if !ok {
			continue
		}
		crit := criterionMap[rec.CriterionID]
		items = append(items, WeeklyItem{
			RecommendationID: recID,
			CriterionID:      rec.CriterionID.String(),
			CriterionName:    crit.Name,
			Type:             rec.Type,
			Title:            rec.Title,
			Weight:           w,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Weight > items[j].Weight
	})
	return items
}

func pickRandomNotificationText(rec model.Recommendation, titleFallback string) string {
	var variants []string
	for _, n := range rec.Notifications {
		if strings.TrimSpace(n.Text) != "" {
			variants = append(variants, n.Text)
		}
	}
	if len(variants) == 0 {
		return titleFallback
	}
	return variants[rand.Intn(len(variants))]
}

// SelectDailyRecommendation picks one recommendation using weighted random selection from the weekly plan.
// Alarms are NOT included in the daily auction.
func (s *HealthService) SelectDailyRecommendation(ctx context.Context, userID uuid.UUID, userSex string) (*DailyRec, error) {
	plan, err := s.GenerateWeeklyRecommendations(ctx, userID, userSex)
	if err != nil {
		return nil, err
	}

	// Filter: only items with weight > 0, exclude alarms.
	type candidate struct {
		item   WeeklyItem
		weight int
	}
	var pool []candidate
	totalWeight := 0
	for _, item := range plan.Items {
		if item.Type == "alarm" || item.Weight <= 0 {
			continue
		}
		pool = append(pool, candidate{item: item, weight: item.Weight})
		totalWeight += item.Weight
	}

	if len(pool) == 0 {
		return &DailyRec{
			Title: "🎉 Рекомендации",
			Text:  "Все ваши показатели в порядке! Продолжайте в том же духе.",
		}, nil
	}

	// Weighted random pick.
	pick := rand.Intn(totalWeight)
	var chosen *candidate
	for i := range pool {
		pick -= pool[i].weight
		if pick < 0 {
			chosen = &pool[i]
			break
		}
	}
	if chosen == nil {
		chosen = &pool[0]
	}

	// Pick a random notification text for the chosen recommendation.
	allRecs := s.cache.GetAllRecommendations()
	text := chosen.item.Title
	for _, r := range allRecs {
		if r.ID.String() == chosen.item.RecommendationID {
			text = pickRandomNotificationText(r, chosen.item.Title)
			break
		}
	}

	// Decrease weight in weekly plan (set to 0 = spent for the week).
	newWeights := make(map[string]int, len(plan.Weights))
	for k, v := range plan.Weights {
		newWeights[k] = v
	}
	newWeights[chosen.item.RecommendationID] = 0
	_ = s.repo.SaveWeeklyWeights(ctx, userID, plan.WeekStart, newWeights)

	return &DailyRec{
		RecommendationID: chosen.item.RecommendationID,
		CriterionID:      chosen.item.CriterionID,
		CriterionName:    chosen.item.CriterionName,
		Title:            chosen.item.Title,
		Text:             text,
		Type:             chosen.item.Type,
	}, nil
}

// GetCachedDailyRecommendation returns today's recommendation from Redis cache, or picks a new one.
func (s *HealthService) GetCachedDailyRecommendation(ctx context.Context, userID uuid.UUID, userSex string) (*DailyRec, error) {
	recKey := "daily_rec:" + userID.String()
	data, err := s.redis.Get(ctx, recKey).Result()
	if err == redis.Nil {
		return s.selectAndCacheDailyRec(ctx, userID, userSex, recKey)
	}
	if err != nil {
		return nil, err
	}
	var rec DailyRec
	if err := json.Unmarshal([]byte(data), &rec); err != nil {
		return s.selectAndCacheDailyRec(ctx, userID, userSex, recKey)
	}
	return &rec, nil
}

func (s *HealthService) selectAndCacheDailyRec(ctx context.Context, userID uuid.UUID, userSex, cacheKey string) (*DailyRec, error) {
	rec, err := s.SelectDailyRecommendation(ctx, userID, userSex)
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(rec)
	s.redis.Set(ctx, cacheKey, string(data), 24*time.Hour)
	return rec, nil
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
	if err := s.repo.CreateNotificationLog(ctx, logEntry); err != nil {
		log.Printf("[notify] CreateNotificationLog user=%s ch=%s: %v", userID, channel, err)
	}

	msg := NotificationMessage{
		UserID:           userID.String(),
		NotificationType: notifType,
		TemplateCode:     templateCode,
		PayloadJSON:      payloadJSON,
	}
	d, _ := json.Marshal(msg)
	subject := "notification." + strings.ToLower(channel)
	if err := s.nc.Publish(subject, d); err != nil {
		log.Printf("[notify] NATS publish subject=%s user=%s: %v", subject, userID, err)
		return err
	}
	log.Printf("[notify] published subject=%s user=%s type=%s", subject, userID, notifType)
	return nil
}

// RunDailyScheduler fires at 08:00 UTC — sends daily recommendation notifications.
func (s *HealthService) RunDailyScheduler(ctx context.Context, channels []string) {
	for {
		next := nextScheduledTime([]int{8})
		log.Printf("[daily] next run scheduled for %s (in %s)", next.Format(time.RFC3339), time.Until(next).Round(time.Second))
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}

		log.Printf("[daily] scheduler fired, fetching users")
		userIDs, err := s.repo.GetAllDistinctUserIDs(ctx)
		if err != nil {
			log.Printf("[daily] GetAllDistinctUserIDs error: %v", err)
			continue
		}
		log.Printf("[daily] sending to %d users via channels %v", len(userIDs), channels)

		for _, userID := range userIDs {
			rec, err := s.selectAndCacheDailyRec(ctx, userID, "", "daily_rec:"+userID.String())
			if err != nil {
				log.Printf("[daily] selectAndCacheDailyRec for %s error: %v", userID, err)
				continue
			}
			payload, _ := json.Marshal(map[string]string{
				"title": rec.Title,
				"body":  rec.Text,
			})
			for _, ch := range channels {
				if err := s.SendNotification(ctx, userID, ch, "daily_recommendation", "daily_rec", string(payload)); err != nil {
					log.Printf("[daily] SendNotification user=%s ch=%s error: %v", userID, ch, err)
				}
			}
		}
		log.Printf("[daily] done")
	}
}

// RunWeeklyScheduler generates weekly recommendation plans every Monday at 00:05 UTC.
func (s *HealthService) RunWeeklyScheduler(ctx context.Context) {
	for {
		next := nextMondayMidnight()
		log.Printf("[weekly] next run scheduled for %s (in %s)", next.Format(time.RFC3339), time.Until(next).Round(time.Second))
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}

		log.Printf("[weekly] scheduler fired")
		userIDs, err := s.repo.GetAllDistinctUserIDs(ctx)
		if err != nil {
			log.Printf("[weekly] GetAllDistinctUserIDs error: %v", err)
			continue
		}
		log.Printf("[weekly] generating plans for %d users", len(userIDs))
		for _, userID := range userIDs {
			if _, err := s.GenerateWeeklyRecommendations(ctx, userID, ""); err != nil {
				log.Printf("[weekly] GenerateWeeklyRecommendations for %s error: %v", userID, err)
			}
		}
		log.Printf("[weekly] done")
	}
}

// RunAlarmScheduler fires at 09:00 and checks for alarm-type recommendations.
func (s *HealthService) RunAlarmScheduler(ctx context.Context, channels []string) {
	for {
		next := nextScheduledTime([]int{9})
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}

		userIDs, err := s.repo.GetAllDistinctUserIDs(ctx)
		if err != nil {
			continue
		}

		allRecs := s.cache.GetAllRecommendations()
		allCriteria := s.cache.GetCriteria()
		criterionMap := make(map[uuid.UUID]model.Criterion)
		for _, c := range allCriteria {
			criterionMap[c.ID] = c
		}

		for _, userID := range userIDs {
			userCriteria, err := s.repo.GetUserCriteria(ctx, userID)
			if err != nil {
				continue
			}
			valueMap := make(map[uuid.UUID]string)
			for _, uc := range userCriteria {
				valueMap[uc.CriterionID] = uc.Value
			}

			for _, rec := range allRecs {
				if rec.Type != "alarm" {
					continue
				}
				crit := criterionMap[rec.CriterionID]
				value := valueMap[rec.CriterionID]
				if !isRecommendationApplicable(rec, crit, value) {
					continue
				}
				dedupeKey := fmt.Sprintf("alarm_notif:%s:%s", userID.String(), rec.ID.String())
				if exists, _ := s.redis.Exists(ctx, dedupeKey).Result(); exists > 0 {
					continue
				}

				text := pickRandomNotificationText(rec, rec.Title)

				payload, _ := json.Marshal(map[string]string{
					"title": rec.Title,
					"body":  text,
				})
				for _, ch := range channels {
					_ = s.SendNotification(ctx, userID, ch, "alarm", "alarm", string(payload))
				}
				s.redis.Set(ctx, dedupeKey, "1", 3*24*time.Hour)
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

// --- Admin ---

func (s *HealthService) AdminListRecommendations(ctx context.Context, criterionID string) ([]model.Recommendation, error) {
	if criterionID != "" {
		cid, err := uuid.Parse(criterionID)
		if err != nil {
			return nil, err
		}
		return s.repo.GetRecommendationsByCriterion(ctx, cid)
	}
	return s.repo.GetAllRecommendations(ctx)
}

func (s *HealthService) AdminUpsertRecommendation(ctx context.Context, rec *model.Recommendation) error {
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	if err := s.repo.UpsertRecommendation(ctx, rec); err != nil {
		return err
	}
	s.cache.refresh(s.repo)
	return nil
}

func (s *HealthService) AdminDeleteRecommendation(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteRecommendation(ctx, id); err != nil {
		return err
	}
	s.cache.refresh(s.repo)
	return nil
}

func (s *HealthService) AdminUpsertCriterion(ctx context.Context, c *model.Criterion) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if err := s.repo.UpsertCriterion(ctx, c); err != nil {
		return err
	}
	s.cache.refresh(s.repo)
	return nil
}

// --- Helpers ---

func ruleWeight(severity string) int {
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

func nextMondayMidnight() time.Time {
	now := time.Now().UTC()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	next := now.AddDate(0, 0, daysUntilMonday)
	return time.Date(next.Year(), next.Month(), next.Day(), 0, 5, 0, 0, time.UTC)
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
	GroupID        string
	Instruction    string
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

type DailyRec struct {
	RecommendationID string `json:"recommendation_id"`
	CriterionID      string `json:"criterion_id"`
	CriterionName    string `json:"criterion_name"`
	Title            string `json:"title"`
	Text             string `json:"text"`
	Type             string `json:"type"`
}

type WeeklyItem struct {
	RecommendationID string
	CriterionID      string
	CriterionName    string
	Type             string
	Title            string
	Weight           int
}

type WeeklyPlan struct {
	WeekStart time.Time
	Items     []WeeklyItem
	Weights   map[string]int
}

