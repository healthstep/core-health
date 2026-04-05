package server

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/service"
	pb "github.com/helthtech/core-health/pkg/proto/health"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HealthServer struct {
	pb.UnimplementedHealthServiceServer
	svc *service.HealthService
}

func NewHealthServer(svc *service.HealthService) *HealthServer {
	return &HealthServer{svc: svc}
}

func (s *HealthServer) ListAnalysis(ctx context.Context, req *pb.ListAnalysisRequest) (*pb.ListAnalysisResponse, error) {
	userID := uuid.Nil
	if req.GetUserId() != "" {
		var err error
		userID, err = uuid.Parse(req.GetUserId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
		}
	}

	analyses, err := s.svc.ListAnalysis(ctx, userID, req.GetUserSex())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list analysis: %v", err)
	}
	resp := &pb.ListAnalysisResponse{}
	for _, a := range analyses {
		resp.Analyses = append(resp.Analyses, &pb.Analysis{
			Id:        a.ID.String(),
			Name:      a.Name,
			Lifetime:  int32(a.Lifetime),
			Sex:       a.Sex,
			BlockedBy: a.BlockedBy,
		})
	}
	return resp, nil
}

func (s *HealthServer) ListCriteria(ctx context.Context, req *pb.ListCriteriaRequest) (*pb.ListCriteriaResponse, error) {
	criteria, err := s.svc.ListCriteria(ctx, req.GetAnalysisId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list criteria: %v", err)
	}
	resp := &pb.ListCriteriaResponse{}
	for _, c := range criteria {
		resp.Criteria = append(resp.Criteria, &pb.Criterion{
			Id:         c.ID.String(),
			Name:       c.Name,
			AnalysisId: c.AnalysisID.String(),
			Level:      int32(c.Level),
		})
	}
	return resp, nil
}

func (s *HealthServer) SetUserCriterion(ctx context.Context, req *pb.SetUserCriterionRequest) (*pb.SetUserCriterionResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	criterionID, err := uuid.Parse(req.GetCriterionId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid criterion_id")
	}
	if err := s.svc.SetUserCriterion(ctx, userID, criterionID, req.GetValue(), req.GetSource()); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("set criterion: %v", err))
	}
	return &pb.SetUserCriterionResponse{Success: true}, nil
}

func (s *HealthServer) ResetAnalysisCriteria(ctx context.Context, req *pb.ResetAnalysisCriteriaRequest) (*pb.ResetAnalysisCriteriaResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	analysisID, err := uuid.Parse(req.GetAnalysisId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid analysis_id")
	}
	if err := s.svc.ResetAnalysisCriteria(ctx, userID, analysisID); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("reset criteria: %v", err))
	}
	return &pb.ResetAnalysisCriteriaResponse{Success: true}, nil
}

func (s *HealthServer) GetUserCriteria(ctx context.Context, req *pb.GetUserCriteriaRequest) (*pb.GetUserCriteriaResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	entries, err := s.svc.GetUserCriteria(ctx, userID, req.GetUserSex())
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("get user criteria: %v", err))
	}
	resp := &pb.GetUserCriteriaResponse{}
	for _, e := range entries {
		resp.Entries = append(resp.Entries, &pb.UserCriterionEntry{
			CriterionId:    e.CriterionID,
			CriterionName:  e.CriterionName,
			AnalysisId:     e.AnalysisID,
			AnalysisName:   e.AnalysisName,
			Value:          e.Value,
			Status:         e.Status,
			Recommendation: e.Recommendation,
			Level:          int32(e.Level),
			Severity:       e.Severity,
		})
	}
	return resp, nil
}

func (s *HealthServer) GetProgress(ctx context.Context, req *pb.GetProgressRequest) (*pb.GetProgressResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	prog, err := s.svc.GetProgress(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("progress: %v", err))
	}
	return &pb.GetProgressResponse{
		Total:      int32(prog.Total),
		Filled:     int32(prog.Filled),
		Percent:    prog.Percent,
		LevelLabel: prog.LevelLabel,
	}, nil
}

func (s *HealthServer) GetRecommendations(ctx context.Context, req *pb.GetRecommendationsRequest) (*pb.GetRecommendationsResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	recs, err := s.svc.GetRecommendations(ctx, userID, req.GetUserSex())
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("recommendations: %v", err))
	}
	resp := &pb.GetRecommendationsResponse{}
	for _, r := range recs {
		resp.Recommendations = append(resp.Recommendations, &pb.Recommendation{
			CriterionId:   r.CriterionID,
			CriterionName: r.CriterionName,
			AnalysisName:  r.AnalysisName,
			Text:          r.Text,
			Severity:      r.Severity,
		})
	}
	return resp, nil
}

func (s *HealthServer) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	if err = s.svc.SendNotification(ctx, userID, req.GetChannel(), req.GetNotificationType(), req.GetTemplateCode(), req.GetPayloadJson()); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("send: %v", err))
	}
	return &pb.SendNotificationResponse{}, nil
}
