package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"github.com/helthtech/core-health/internal/pdfextract"
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

func (s *HealthServer) ListGroups(ctx context.Context, _ *pb.ListGroupsRequest) (*pb.ListGroupsResponse, error) {
	groups, err := s.svc.ListGroups(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list groups: %v", err)
	}
	resp := &pb.ListGroupsResponse{}
	for _, g := range groups {
		resp.Groups = append(resp.Groups, &pb.CriterionGroup{
			Id:        g.ID.String(),
			Name:      g.Name,
			SortOrder: int32(g.SortOrder),
		})
	}
	return resp, nil
}

func (s *HealthServer) ListCriteria(ctx context.Context, req *pb.ListCriteriaRequest) (*pb.ListCriteriaResponse, error) {
	userID := uuid.Nil
	if req.GetUserId() != "" {
		var err error
		userID, err = uuid.Parse(req.GetUserId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
		}
	}

	criteria, err := s.svc.ListCriteria(ctx, userID, req.GetUserSex())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list criteria: %v", err)
	}
	resp := &pb.ListCriteriaResponse{}
	for _, c := range criteria {
		pc := criterionToProto(c)
		resp.Criteria = append(resp.Criteria, pc)
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
	if err := s.svc.SetUserCriterion(ctx, userID, criterionID, req.GetValue(), req.GetSource(), req.GetMeasuredAt()); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("set criterion: %v", err))
	}
	return &pb.SetUserCriterionResponse{Success: true}, nil
}

func (s *HealthServer) ResetCriteria(ctx context.Context, req *pb.ResetCriteriaRequest) (*pb.ResetCriteriaResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	if err := s.svc.ResetAllCriteria(ctx, userID); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("reset criteria: %v", err))
	}
	return &pb.ResetCriteriaResponse{Success: true}, nil
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
			Value:          e.Value,
			Status:         e.Status,
			Recommendation: e.Recommendation,
			Level:          int32(e.Level),
			Severity:       e.Severity,
			InputType:      e.InputType,
			GroupId:        e.GroupID,
			Instruction:    e.Instruction,
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

func (s *HealthServer) ImportCriteriaFromPdf(stream pb.HealthService_ImportCriteriaFromPdfServer) error {
	var buf bytes.Buffer
	var filename string
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if fn := msg.GetFilename(); fn != "" {
			filename = fn
		}
		chunk := msg.GetChunk()
		if len(chunk) == 0 {
			continue
		}
		if buf.Len()+len(chunk) > pdfextract.MaxPDFBytes {
			return status.Errorf(codes.ResourceExhausted, "pdf exceeds max size (%d bytes)", pdfextract.MaxPDFBytes)
		}
		buf.Write(chunk)
	}
	if buf.Len() == 0 {
		return status.Error(codes.InvalidArgument, "empty pdf stream")
	}
	text, err := pdfextract.PlainText(buf.Bytes())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "pdf parse: %v", err)
	}
	logPDFImportText(filename, buf.Len(), text)
	return stream.SendAndClose(&pb.ImportCriteriaFromPdfResponse{UserCriteria: nil})
}

const maxPDFLogRunes = 12000

func logPDFImportText(filename string, sizeBytes int, text string) {
	runes := []rune(text)
	n := len(runes)
	trunc := n > maxPDFLogRunes
	if trunc {
		runes = runes[:maxPDFLogRunes]
	}
	suffix := ""
	if trunc {
		suffix = " (log truncated)"
	}
	log.Printf("[pdf-import] filename=%q size_bytes=%d text_runes=%d%s\n%s",
		filename, sizeBytes, n, suffix, string(runes))
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
			Text:          r.Text,
			Severity:      r.Severity,
		})
	}
	return resp, nil
}

func (s *HealthServer) GetWeeklyRecommendations(ctx context.Context, req *pb.GetWeeklyRecommendationsRequest) (*pb.GetWeeklyRecommendationsResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user_id")
	}
	plan, err := s.svc.GenerateWeeklyRecommendations(ctx, userID, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("weekly recommendations: %v", err))
	}
	resp := &pb.GetWeeklyRecommendationsResponse{
		WeekStart: plan.WeekStart.Format("2006-01-02"),
	}
	for _, item := range plan.Items {
		resp.Items = append(resp.Items, &pb.WeeklyRecommendationItem{
			RecommendationId: item.RecommendationID,
			CriterionId:      item.CriterionID,
			CriterionName:    item.CriterionName,
			Type:             item.Type,
			Title:            item.Title,
			Weight:           int32(item.Weight),
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

// --- Admin ---

func (s *HealthServer) AdminListRecommendations(ctx context.Context, req *pb.AdminListRecommendationsRequest) (*pb.AdminListRecommendationsResponse, error) {
	recs, err := s.svc.AdminListRecommendations(ctx, req.GetCriterionId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list recommendations: %v", err)
	}
	resp := &pb.AdminListRecommendationsResponse{}
	for _, r := range recs {
		resp.Recommendations = append(resp.Recommendations, modelRecToProto(r))
	}
	return resp, nil
}

func (s *HealthServer) AdminUpsertRecommendation(ctx context.Context, req *pb.AdminUpsertRecommendationRequest) (*pb.AdminUpsertRecommendationResponse, error) {
	pr := req.GetRecommendation()
	if pr == nil {
		return nil, status.Errorf(codes.InvalidArgument, "recommendation is required")
	}
	rec, err := protoRecToModel(pr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := s.svc.AdminUpsertRecommendation(ctx, rec); err != nil {
		return nil, status.Errorf(codes.Internal, "upsert recommendation: %v", err)
	}
	return &pb.AdminUpsertRecommendationResponse{Recommendation: modelRecToProto(*rec)}, nil
}

func (s *HealthServer) AdminDeleteRecommendation(ctx context.Context, req *pb.AdminDeleteRecommendationRequest) (*pb.AdminDeleteRecommendationResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id")
	}
	if err := s.svc.AdminDeleteRecommendation(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete recommendation: %v", err)
	}
	return &pb.AdminDeleteRecommendationResponse{Success: true}, nil
}

func (s *HealthServer) AdminUpsertCriterion(ctx context.Context, req *pb.AdminUpsertCriterionRequest) (*pb.AdminUpsertCriterionResponse, error) {
	pc := req.GetCriterion()
	if pc == nil {
		return nil, status.Errorf(codes.InvalidArgument, "criterion is required")
	}
	c := &model.Criterion{
		Name:        pc.GetName(),
		Level:       int(pc.GetLevel()),
		Sex:         pc.GetSex(),
		InputType:   pc.GetInputType(),
		Lifetime:    int(pc.GetLifetime()),
		SortOrder:   int(pc.GetSortOrder()),
		MinValue:    pc.MinValue,
		MaxValue:    pc.MaxValue,
		Delta:       pc.Delta,
		Instruction: pc.GetInstruction(),
	}
	if pc.GetId() != "" {
		id, err := uuid.Parse(pc.GetId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid id")
		}
		c.ID = id
	}
	if pc.GetGroupId() != "" {
		gid, err := uuid.Parse(pc.GetGroupId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid group_id")
		}
		c.GroupID = &gid
	}
	if err := s.svc.AdminUpsertCriterion(ctx, c); err != nil {
		return nil, status.Errorf(codes.Internal, "upsert criterion: %v", err)
	}
	return &pb.AdminUpsertCriterionResponse{Criterion: criterionToProto(*c)}, nil
}

// --- Helpers ---

func criterionToProto(c model.Criterion) *pb.Criterion {
	pc := &pb.Criterion{
		Id:          c.ID.String(),
		Name:        c.Name,
		Level:       int32(c.Level),
		Sex:         c.Sex,
		InputType:   c.InputType,
		Lifetime:    int32(c.Lifetime),
		SortOrder:   int32(c.SortOrder),
		MinValue:    c.MinValue,
		MaxValue:    c.MaxValue,
		Delta:       c.Delta,
		Instruction: c.Instruction,
	}
	if c.GroupID != nil {
		pc.GroupId = c.GroupID.String()
	}
	return pc
}

func modelRecToProto(r model.Recommendation) *pb.AdminRecommendation {
	texts := make([]string, 0, len(r.Notifications))
	for _, n := range r.Notifications {
		texts = append(texts, n.Text)
	}
	return &pb.AdminRecommendation{
		Id:          r.ID.String(),
		CriterionId: r.CriterionID.String(),
		Type:        r.Type,
		Title:       r.Title,
		Texts:       texts,
		BaseWeight:  int32(r.BaseWeight),
	}
}

func protoRecToModel(pr *pb.AdminRecommendation) (*model.Recommendation, error) {
	criterionID, err := uuid.Parse(pr.GetCriterionId())
	if err != nil {
		return nil, fmt.Errorf("invalid criterion_id: %w", err)
	}
	rec := &model.Recommendation{
		CriterionID: criterionID,
		Type:        pr.GetType(),
		Title:       pr.GetTitle(),
		BaseWeight:  int(pr.GetBaseWeight()),
	}
	if pr.GetId() != "" {
		id, err := uuid.Parse(pr.GetId())
		if err != nil {
			return nil, fmt.Errorf("invalid id: %w", err)
		}
		rec.ID = id
	}
	for _, t := range pr.GetTexts() {
		if strings.TrimSpace(t) == "" {
			continue
		}
		rec.Notifications = append(rec.Notifications, model.RecommendationNotification{Text: t})
	}
	return rec, nil
}
