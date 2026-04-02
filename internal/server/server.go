package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"github.com/helthtech/core-health/internal/service"
	pb "github.com/helthtech/core-health/pkg/proto/health"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type HealthServer struct {
	pb.UnimplementedHealthServiceServer
	svc *service.HealthService
}

func NewHealthServer(svc *service.HealthService) *HealthServer {
	return &HealthServer{svc: svc}
}

func (s *HealthServer) ListCriteria(ctx context.Context, _ *pb.ListCriteriaRequest) (*pb.ListCriteriaResponse, error) {
	criteria, err := s.svc.ListCriteria(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list criteria: %v", err)
	}
	resp := &pb.ListCriteriaResponse{}
	for _, c := range criteria {
		resp.Criteria = append(resp.Criteria, &pb.Criterion{
			Id:                     c.ID.String(),
			Code:                   c.Code,
			Name:                   c.Name,
			Description:            c.Description,
			ValueType:              c.ValueType,
			Unit:                   c.Unit,
			InputModes:             splitCSV(c.InputModes),
			RecurrenceIntervalDays: int32(c.RecurrenceIntervalDays),
			IsActive:               c.IsActive,
		})
	}
	return resp, nil
}

func (s *HealthServer) ListLabTests(ctx context.Context, _ *pb.ListLabTestsRequest) (*pb.ListLabTestsResponse, error) {
	tests, err := s.svc.ListLabTests(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list lab tests: %v", err)
	}
	resp := &pb.ListLabTestsResponse{}
	for _, t := range tests {
		links, _ := s.svc.GetLabTestCriteria(ctx, t.ID)
		ids := make([]string, 0, len(links))
		for _, l := range links {
			ids = append(ids, l.HealthCriterionID.String())
		}
		resp.LabTests = append(resp.LabTests, &pb.LabTest{
			Id:           t.ID.String(),
			Code:         t.Code,
			Name:         t.Name,
			Description:  t.Description,
			CriterionIds: ids,
		})
	}
	return resp, nil
}

func (s *HealthServer) CreateNumericEvent(ctx context.Context, req *pb.CreateNumericEventRequest) (*pb.EventResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	criterionID, err := uuid.Parse(req.HealthCriterionId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid criterion_id")
	}
	var labTestID *uuid.UUID
	if req.LabTestId != "" {
		id, e := uuid.Parse(req.LabTestId)
		if e != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid lab_test_id")
		}
		labTestID = &id
	}
	occurredAt := time.Now()
	if req.OccurredAt != nil {
		occurredAt = req.OccurredAt.AsTime()
	}
	e, err := s.svc.CreateNumericEvent(ctx, userID, criterionID, labTestID, req.NumericValue, occurredAt, req.Source, req.Note)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("create: %v", err))
	}
	return modelEventToProto(e), nil
}

func (s *HealthServer) CreateBooleanEvent(ctx context.Context, req *pb.CreateBooleanEventRequest) (*pb.EventResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	criterionID, err := uuid.Parse(req.HealthCriterionId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid criterion_id")
	}
	occurredAt := time.Now()
	if req.OccurredAt != nil {
		occurredAt = req.OccurredAt.AsTime()
	}
	e, err := s.svc.CreateBooleanEvent(ctx, userID, criterionID, req.BooleanValue, occurredAt, req.Source, req.Note)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("create: %v", err))
	}
	return modelEventToProto(e), nil
}

func (s *HealthServer) CreateMarkDoneEvent(ctx context.Context, req *pb.CreateMarkDoneEventRequest) (*pb.EventResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	criterionID, err := uuid.Parse(req.HealthCriterionId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid criterion_id")
	}
	occurredAt := time.Now()
	if req.OccurredAt != nil {
		occurredAt = req.OccurredAt.AsTime()
	}
	e, err := s.svc.CreateMarkDoneEvent(ctx, userID, criterionID, occurredAt, req.Source, req.Note)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("create: %v", err))
	}
	return modelEventToProto(e), nil
}

func (s *HealthServer) CreateDocument(ctx context.Context, req *pb.CreateDocumentRequest) (*pb.DocumentResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	d, err := s.svc.CreateDocument(ctx, userID, req.StorageKey, req.MimeType, req.ByteSize, req.Sha256)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("create doc: %v", err))
	}
	return &pb.DocumentResponse{
		Id:         d.ID.String(),
		UserId:     d.UserID.String(),
		StorageKey: d.StorageKey,
		CreatedAt:  timestamppb.New(d.CreatedAt),
	}, nil
}

func (s *HealthServer) GetDashboard(ctx context.Context, req *pb.GetDashboardRequest) (*pb.DashboardResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	d, err := s.svc.GetDashboard(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("dashboard: %v", err))
	}
	resp := &pb.DashboardResponse{
		Level:           d.Level,
		ProgressPercent: d.ProgressPercent,
		TotalCriteria:   int32(d.TotalCriteria),
		FilledCriteria:  int32(d.FilledCriteria),
		OverdueCriteria: int32(d.OverdueCriteria),
	}
	for _, cs := range d.States {
		st := &pb.CriterionState{
			CriterionId:      cs.CriterionID,
			CriterionName:    cs.CriterionName,
			Status:           cs.Status,
			LastValueSummary: cs.LastValue,
			Recommendation:   cs.Recommendation,
		}
		if cs.LastEventAt != nil {
			st.LastEventAt = timestamppb.New(*cs.LastEventAt)
		}
		resp.States = append(resp.States, st)
	}
	return resp, nil
}

func (s *HealthServer) GetUserCriterionStates(ctx context.Context, req *pb.GetUserCriterionStatesRequest) (*pb.GetUserCriterionStatesResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	states, err := s.svc.GetUserCriterionStates(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("states: %v", err))
	}
	resp := &pb.GetUserCriterionStatesResponse{}
	for _, st := range states {
		cs := &pb.CriterionState{
			CriterionId:      st.HealthCriterionID.String(),
			Status:           st.Status,
			LastValueSummary: st.LastValue,
		}
		if st.LastEventAt != nil {
			cs.LastEventAt = timestamppb.New(*st.LastEventAt)
		}
		resp.States = append(resp.States, cs)
	}
	return resp, nil
}

func (s *HealthServer) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	if err = s.svc.SendNotification(ctx, userID, req.Channel, req.NotificationType, req.TemplateCode, req.PayloadJson); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("send: %v", err))
	}
	return &pb.SendNotificationResponse{}, nil
}

func modelEventToProto(e *model.UserCriterionEvent) *pb.EventResponse {
	resp := &pb.EventResponse{
		Id:         e.ID.String(),
		UserId:     e.UserID.String(),
		EventType:  e.EventType,
		OccurredAt: timestamppb.New(e.OccurredAt),
		RecordedAt: timestamppb.New(e.RecordedAt),
	}
	if e.HealthCriterionID != nil {
		resp.HealthCriterionId = e.HealthCriterionID.String()
	}
	return resp
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}
