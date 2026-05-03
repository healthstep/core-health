package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/labimport"
	"github.com/helthtech/core-health/internal/model"
	"github.com/helthtech/core-health/internal/pdfextract"
	"github.com/helthtech/core-health/internal/service"
	pb "github.com/helthtech/core-health/pkg/proto/health"
	criteriapb "github.com/porebric/creteria_parser/pkg/proto/criteria"
	"github.com/porebric/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type HealthServer struct {
	pb.UnimplementedHealthServiceServer
	svc    *service.HealthService
	parser criteriapb.CriteriaParserClient
	lab    *labimport.Store
}

func NewHealthServer(svc *service.HealthService, parser criteriapb.CriteriaParserClient, lab *labimport.Store) *HealthServer {
	return &HealthServer{svc: svc, parser: parser, lab: lab}
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
	out := &pb.SetUserCriterionResponse{Success: true}
	if entry, err := s.svc.BuildUserCriterionEntryAfterSet(ctx, criterionID, req.GetUserSex(), req.GetValue()); err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("set criterion: %v", err))
	} else if entry != nil {
		out.Entry = userCriterionEntryToProto(*entry)
	}
	return out, nil
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
		resp.Entries = append(resp.Entries, userCriterionEntryToProto(e))
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
	ctx := stream.Context()
	if s.parser == nil || s.lab == nil {
		logger.Error(ctx, fmt.Errorf("import pdf: dependencies missing"), "import pdf: missing deps",
			"parser_nil", s.parser == nil, "lab_nil", s.lab == nil)
		return status.Error(codes.Unavailable, "lab import: criteria parser or redis not configured")
	}
	type fileAcc struct {
		name string
		buf  bytes.Buffer
	}
	var files []fileAcc
	var cur *fileAcc
	var filename, userIDStr, userSex string
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Error(ctx, err, "import pdf: recv error")
			return err
		}
		if id := msg.GetUserId(); id != "" {
			userIDStr = id
		}
		if sx := msg.GetUserSex(); sx != "" {
			userSex = sx
		}
		if fn := msg.GetFilename(); fn != "" {
			if cur != nil && fn != cur.name && cur.buf.Len() > 0 {
				files = append(files, *cur)
				cur = &fileAcc{name: fn}
			} else if cur == nil {
				cur = &fileAcc{name: fn}
			} else if fn != cur.name {
				cur = &fileAcc{name: fn}
			}
			filename = fn
		}
		chunk := msg.GetChunk()
		if len(chunk) == 0 {
			continue
		}
		if cur == nil {
			cur = &fileAcc{name: filename}
		}
		if cur.buf.Len()+len(chunk) > pdfextract.MaxPDFBytes {
			logger.Error(ctx, fmt.Errorf("size limit"), "import pdf: size limit", "max", pdfextract.MaxPDFBytes)
			return status.Errorf(codes.ResourceExhausted, "pdf exceeds max size (%d bytes)", pdfextract.MaxPDFBytes)
		}
		cur.buf.Write(chunk)
	}
	if cur != nil && cur.buf.Len() > 0 {
		files = append(files, *cur)
	}
	var total int
	for i := range files {
		total += files[i].buf.Len()
	}
	if total == 0 {
		logger.Error(ctx, fmt.Errorf("empty stream"), "import pdf: empty stream")
		return status.Error(codes.InvalidArgument, "empty pdf stream")
	}
	if userIDStr == "" {
		logger.Error(ctx, fmt.Errorf("missing user_id"), "import pdf: missing user_id")
		return status.Error(codes.InvalidArgument, "user_id is required in the first stream message")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		logger.Error(ctx, err, "import pdf: invalid user_id")
		return status.Error(codes.InvalidArgument, "invalid user_id")
	}
	var textParts []string
	for _, f := range files {
		t, err := pdfextract.PlainText(f.buf.Bytes())
		if err != nil {
			logger.Error(ctx, err, "import pdf: extract text", "file", f.name)
			return status.Errorf(codes.InvalidArgument, "pdf parse %q: %v", f.name, err)
		}
		textParts = append(textParts, t)
	}
	text := strings.Join(textParts, "\n\n---\n\n")
	logPDFImportText(ctx, filename, total, text)

	criteria, err := s.svc.ListCriteria(ctx, userID, userSex)
	if err != nil {
		logger.Error(ctx, err, "import pdf: list criteria", "user_id", userIDStr)
		return status.Errorf(codes.Internal, "list criteria: %v", err)
	}
	var hints []*criteriapb.CriterionHint
	cat := make(map[uuid.UUID]model.Criterion)
	for _, c := range criteria {
		it := strings.ToLower(c.InputType)
		if it != "numeric" && it != "boolean" && it != "check" {
			continue
		}
		cat[c.ID] = c
		h := &criteriapb.CriterionHint{
			Id:       c.ID.String(),
			Name:     c.Name,
			UnitHint: s.svc.AnalysisInstructionForCriterion(c),
		}
		// go pb optional fields
		if c.InputType != "" {
			h.InputType = c.InputType
		}
		if c.MinValue != nil {
			h.MinValue = c.MinValue
		}
		if c.MaxValue != nil {
			h.MaxValue = c.MaxValue
		}
		hints = append(hints, h)
	}
	if len(hints) == 0 {
		return stream.SendAndClose(&pb.ImportCriteriaFromPdfResponse{})
	}

	pr, err := s.parser.ParseFromText(ctx, &criteriapb.ParseFromTextRequest{
		Text:            text,
		AllowedCriteria: hints,
		UserSex:         userSex,
		Locale:          "ru",
	})
	if err != nil {
		logger.Error(ctx, err, "import pdf: parser", "user_id", userIDStr)
		return status.Errorf(codes.Internal, "ai parse: %v", err)
	}

	var out []*pb.ImportedUserCriterion
	var items []labimport.PendingItem
	for _, r := range pr.GetResults() {
		cid, err := uuid.Parse(r.GetCriterionId())
		if err != nil {
			continue
		}
		c, ok := cat[cid]
		if !ok {
			continue
		}
		inst := s.svc.AnalysisInstructionForCriterion(c)
		out = append(out, &pb.ImportedUserCriterion{
			CriterionId:   c.ID.String(),
			CriterionName: c.Name,
			Value:         r.GetValue(),
			InputType:     c.InputType,
			MeasuredAt:    r.GetMeasuredAt(),
			Instruction:   inst,
		})
		items = append(items, labimport.PendingItem{
			CriterionID:   c.ID.String(),
			CriterionName: c.Name,
			Value:         r.GetValue(),
			InputType:     c.InputType,
			MeasuredAt:    r.GetMeasuredAt(),
			Instruction:   inst,
		})
	}

	if len(out) == 0 {
		return stream.SendAndClose(&pb.ImportCriteriaFromPdfResponse{
			ModelNote: pr.GetModelNote(),
		})
	}

	pendingID := uuid.NewString()
	if err := s.lab.Save(ctx, pendingID, labimport.PendingBatch{UserID: userIDStr, Items: items}, time.Hour); err != nil {
		logger.Error(ctx, err, "import pdf: redis save", "pending_id", pendingID)
		return status.Errorf(codes.Internal, "redis: %v", err)
	}

	logger.Info(ctx, "import pdf: success", "user_id", userIDStr, "files", len(files), "items", len(out), "pending_id", pendingID)
	return stream.SendAndClose(&pb.ImportCriteriaFromPdfResponse{
		UserCriteria:    out,
		PendingImportId: pendingID,
		ModelNote:       pr.GetModelNote(),
	})
}

func (s *HealthServer) ConfirmPendingImport(ctx context.Context, req *pb.ConfirmPendingImportRequest) (*pb.ConfirmPendingImportResponse, error) {
	if s.lab == nil {
		return &pb.ConfirmPendingImportResponse{Success: false, ErrorMessage: "redis not configured"}, nil
	}
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}
	if req.GetPendingId() == "" {
		return nil, status.Error(codes.InvalidArgument, "pending_id required")
	}
	batch, err := s.lab.Load(ctx, req.GetPendingId())
	if err != nil {
		return &pb.ConfirmPendingImportResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	if batch.UserID != req.GetUserId() {
		_ = s.lab.Delete(ctx, req.GetPendingId())
		return &pb.ConfirmPendingImportResponse{Success: false, ErrorMessage: "forbidden"}, nil
	}
	if !req.GetAccept() {
		_ = s.lab.Delete(ctx, req.GetPendingId())
		return &pb.ConfirmPendingImportResponse{Success: true}, nil
	}
	var n int32
	for _, it := range batch.Items {
		cid, err := uuid.Parse(it.CriterionID)
		if err != nil {
			continue
		}
		if err := s.svc.SetUserCriterion(ctx, userID, cid, it.Value, "import_ai", it.MeasuredAt); err != nil {
			_ = s.lab.Delete(ctx, req.GetPendingId())
			return &pb.ConfirmPendingImportResponse{Success: false, ErrorMessage: err.Error(), Applied: n}, nil
		}
		n++
	}
	_ = s.lab.Delete(ctx, req.GetPendingId())
	return &pb.ConfirmPendingImportResponse{Success: true, Applied: n}, nil
}

const maxPDFLogRunes = 12000

func logPDFImportText(ctx context.Context, filename string, sizeBytes int, text string) {
	runes := []rune(text)
	n := len(runes)
	trunc := n > maxPDFLogRunes
	if trunc {
		runes = runes[:maxPDFLogRunes]
	}
	s := string(runes)
	if trunc {
		s += "...(truncated)"
	}
	logger.Info(ctx, "pdf import extracted text", "filename", filename, "size_bytes", sizeBytes, "text_runes", n, "text_preview", s)
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
		Name:      pc.GetName(),
		Level:     int(pc.GetLevel()),
		Sex:       pc.GetSex(),
		InputType: pc.GetInputType(),
		Lifetime:  int(pc.GetLifetime()),
		SortOrder: int(pc.GetSortOrder()),
		MinValue:  pc.MinValue,
		MaxValue:  pc.MaxValue,
		Delta:     pc.Delta,
	}
	if pc.AnalysisId != nil {
		aid := pc.GetAnalysisId()
		c.AnalysisID = &aid
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

func (s *HealthServer) ListAnalyses(ctx context.Context, _ *pb.ListAnalysesRequest) (*pb.ListAnalysesResponse, error) {
	list, err := s.svc.ListAnalyses(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list analyses: %v", err)
	}
	resp := &pb.ListAnalysesResponse{}
	for _, a := range list {
		resp.Analyses = append(resp.Analyses, analysisToProto(a))
	}
	return resp, nil
}

func (s *HealthServer) GetAnalysis(ctx context.Context, req *pb.GetAnalysisRequest) (*pb.GetAnalysisResponse, error) {
	a, err := s.svc.GetAnalysis(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Errorf(codes.NotFound, "analysis not found")
		}
		return nil, status.Errorf(codes.Internal, "get analysis: %v", err)
	}
	return &pb.GetAnalysisResponse{Analysis: analysisToProto(*a)}, nil
}

func (s *HealthServer) AdminListAnalyses(ctx context.Context, _ *pb.AdminListAnalysesRequest) (*pb.AdminListAnalysesResponse, error) {
	list, err := s.svc.AdminListAnalyses(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list analyses: %v", err)
	}
	resp := &pb.AdminListAnalysesResponse{}
	for _, a := range list {
		resp.Analyses = append(resp.Analyses, analysisToProto(a))
	}
	return resp, nil
}

func (s *HealthServer) AdminUpsertAnalysis(ctx context.Context, req *pb.AdminUpsertAnalysisRequest) (*pb.AdminUpsertAnalysisResponse, error) {
	pa := req.GetAnalysis()
	if pa == nil {
		return nil, status.Errorf(codes.InvalidArgument, "analysis is required")
	}
	a := &model.Analysis{
		Name:        pa.GetName(),
		Instruction: pa.GetInstruction(),
	}
	if pa.GetId() != 0 {
		a.ID = pa.GetId()
	}
	if err := s.svc.AdminUpsertAnalysis(ctx, a); err != nil {
		return nil, status.Errorf(codes.Internal, "upsert analysis: %v", err)
	}
	return &pb.AdminUpsertAnalysisResponse{Analysis: analysisToProto(*a)}, nil
}

func (s *HealthServer) AdminDeleteAnalysis(ctx context.Context, req *pb.AdminDeleteAnalysisRequest) (*pb.AdminDeleteAnalysisResponse, error) {
	if err := s.svc.AdminDeleteAnalysis(ctx, req.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete analysis: %v", err)
	}
	return &pb.AdminDeleteAnalysisResponse{Success: true}, nil
}

// --- Helpers ---

func userCriterionEntryToProto(e service.UserCriterionEntry) *pb.UserCriterionEntry {
	out := &pb.UserCriterionEntry{
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
		AnalysisName:   e.AnalysisName,
	}
	if e.AnalysisID != 0 {
		aid := e.AnalysisID
		out.AnalysisId = &aid
	}
	return out
}

func criterionToProto(c model.Criterion) *pb.Criterion {
	pc := &pb.Criterion{
		Id:        c.ID.String(),
		Name:      c.Name,
		Level:     int32(c.Level),
		Sex:       c.Sex,
		InputType: c.InputType,
		Lifetime:  int32(c.Lifetime),
		SortOrder: int32(c.SortOrder),
		MinValue:  c.MinValue,
		MaxValue:  c.MaxValue,
		Delta:     c.Delta,
	}
	if c.GroupID != nil {
		pc.GroupId = c.GroupID.String()
	}
	if c.AnalysisID != nil {
		aid := *c.AnalysisID
		pc.AnalysisId = &aid
	}
	return pc
}

func analysisToProto(a model.Analysis) *pb.Analysis {
	return &pb.Analysis{
		Id:           a.ID,
		Name:         a.Name,
		Instruction:  a.Instruction,
	}
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
