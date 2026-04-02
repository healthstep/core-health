package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"github.com/helthtech/core-health/internal/repository"
	"github.com/nats-io/nats.go"
)

type HealthService struct {
	repo *repository.HealthRepository
	nc   *nats.Conn
}

func NewHealthService(repo *repository.HealthRepository, nc *nats.Conn) *HealthService {
	return &HealthService{repo: repo, nc: nc}
}

func (s *HealthService) ListCriteria(ctx context.Context) ([]model.HealthCriterion, error) {
	return s.repo.ListCriteria(ctx)
}

func (s *HealthService) ListLabTests(ctx context.Context) ([]model.LabTest, error) {
	return s.repo.ListLabTests(ctx)
}

func (s *HealthService) GetLabTestCriteria(ctx context.Context, labTestID uuid.UUID) ([]model.LabTestCriterion, error) {
	return s.repo.GetLabTestCriteria(ctx, labTestID)
}

func (s *HealthService) CreateNumericEvent(ctx context.Context, userID, criterionID uuid.UUID, labTestID *uuid.UUID, value float64, occurredAt time.Time, source, note string) (*model.UserCriterionEvent, error) {
	e := &model.UserCriterionEvent{
		UserID:            userID,
		HealthCriterionID: &criterionID,
		LabTestID:         labTestID,
		EventType:         "numeric_entry",
		NumericValue:      &value,
		OccurredAt:        occurredAt,
		Source:            source,
		Note:              note,
	}
	if err := s.repo.CreateEvent(ctx, e); err != nil {
		return nil, fmt.Errorf("create numeric event: %w", err)
	}
	s.updateCriterionState(ctx, userID, criterionID, occurredAt, fmt.Sprintf("%.2f", value))
	return e, nil
}

func (s *HealthService) CreateBooleanEvent(ctx context.Context, userID, criterionID uuid.UUID, boolValue string, occurredAt time.Time, source, note string) (*model.UserCriterionEvent, error) {
	e := &model.UserCriterionEvent{
		UserID:            userID,
		HealthCriterionID: &criterionID,
		EventType:         "boolean_entry",
		BooleanValue:      boolValue,
		OccurredAt:        occurredAt,
		Source:            source,
		Note:              note,
	}
	if err := s.repo.CreateEvent(ctx, e); err != nil {
		return nil, fmt.Errorf("create boolean event: %w", err)
	}
	s.updateCriterionState(ctx, userID, criterionID, occurredAt, boolValue)
	return e, nil
}

func (s *HealthService) CreateMarkDoneEvent(ctx context.Context, userID, criterionID uuid.UUID, occurredAt time.Time, source, note string) (*model.UserCriterionEvent, error) {
	e := &model.UserCriterionEvent{
		UserID:            userID,
		HealthCriterionID: &criterionID,
		EventType:         "marked_done",
		OccurredAt:        occurredAt,
		Source:            source,
		Note:              note,
	}
	if err := s.repo.CreateEvent(ctx, e); err != nil {
		return nil, fmt.Errorf("create mark_done event: %w", err)
	}
	s.updateCriterionState(ctx, userID, criterionID, occurredAt, "done")
	return e, nil
}

func (s *HealthService) CreateDocument(ctx context.Context, userID uuid.UUID, storageKey, mimeType string, byteSize int64, sha256 string) (*model.Document, error) {
	d := &model.Document{
		ID:         uuid.New(),
		UserID:     userID,
		StorageKey: storageKey,
		MimeType:   mimeType,
		ByteSize:   byteSize,
		SHA256:     sha256,
	}
	if err := s.repo.CreateDocument(ctx, d); err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}
	return d, nil
}

func (s *HealthService) GetDashboard(ctx context.Context, userID uuid.UUID) (*Dashboard, error) {
	criteria, err := s.repo.ListCriteria(ctx)
	if err != nil {
		return nil, err
	}
	states, err := s.repo.GetUserCriterionStates(ctx, userID)
	if err != nil {
		return nil, err
	}

	stateMap := make(map[uuid.UUID]*model.UserCriterionState)
	for i := range states {
		stateMap[states[i].HealthCriterionID] = &states[i]
	}

	d := &Dashboard{
		TotalCriteria: len(criteria),
	}
	for _, c := range criteria {
		cs := CriterionStatus{
			CriterionID:   c.ID.String(),
			CriterionName: c.Name,
			Status:        "missing",
		}
		if st, ok := stateMap[c.ID]; ok {
			cs.Status = st.Status
			cs.LastEventAt = st.LastEventAt
			cs.LastValue = st.LastValue
			if st.Status == "ok" || st.Status == "warning" {
				d.FilledCriteria++
			}
			if st.Status == "overdue" {
				d.OverdueCriteria++
			}
		}
		d.States = append(d.States, cs)
	}

	d.ProgressPercent = 0
	if d.TotalCriteria > 0 {
		d.ProgressPercent = float64(d.FilledCriteria) / float64(d.TotalCriteria) * 100
	}
	d.Level = computeLevel(d.ProgressPercent)

	return d, nil
}

func (s *HealthService) GetUserCriterionStates(ctx context.Context, userID uuid.UUID) ([]model.UserCriterionState, error) {
	return s.repo.GetUserCriterionStates(ctx, userID)
}

type NotificationMessage struct {
	UserID           string `json:"user_id"`
	NotificationType string `json:"notification_type"`
	TemplateCode     string `json:"template_code"`
	PayloadJSON      string `json:"payload_json"`
}

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
	data, _ := json.Marshal(msg)

	subject := "notification." + strings.ToLower(channel)
	return s.nc.Publish(subject, data)
}

func (s *HealthService) updateCriterionState(ctx context.Context, userID, criterionID uuid.UUID, eventAt time.Time, value string) {
	states, _ := s.repo.GetUserCriterionStates(ctx, userID)
	for _, st := range states {
		if st.HealthCriterionID == criterionID {
			st.LastEventAt = &eventAt
			st.LastValue = value
			st.Status = "ok"
			st.EscalationStep = 0
			_ = s.repo.UpsertCriterionState(ctx, &st)
			return
		}
	}
	newState := &model.UserCriterionState{
		ID:                uuid.New(),
		UserID:            userID,
		HealthCriterionID: criterionID,
		LastEventAt:       &eventAt,
		LastValue:         value,
		Status:            "ok",
	}
	_ = s.repo.UpsertCriterionState(ctx, newState)
}

type Dashboard struct {
	Level           string
	ProgressPercent float64
	TotalCriteria   int
	FilledCriteria  int
	OverdueCriteria int
	States          []CriterionStatus
}

type CriterionStatus struct {
	CriterionID    string
	CriterionName  string
	Status         string
	LastEventAt    *time.Time
	LastValue      string
	Recommendation string
}

func computeLevel(pct float64) string {
	switch {
	case pct >= 80:
		return "full_control"
	case pct >= 50:
		return "overachiever"
	default:
		return "normie"
	}
}
