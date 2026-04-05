package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"github.com/helthtech/core-health/internal/repository"
)

// AnalysisCache holds in-memory snapshots of analyses and criteria,
// refreshed from the database every minute.
type AnalysisCache struct {
	mu               sync.RWMutex
	analyses         []model.Analysis
	criteria         []model.Criterion
	analysisMap      map[uuid.UUID]model.Analysis  // analysis_id → Analysis
	criterionMap     map[uuid.UUID]model.Criterion // criterion_id → Criterion
	rulesByCriterion map[uuid.UUID][]model.RecommendationRule
}

func NewAnalysisCache() *AnalysisCache {
	return &AnalysisCache{
		analysisMap:      make(map[uuid.UUID]model.Analysis),
		criterionMap:     make(map[uuid.UUID]model.Criterion),
		rulesByCriterion: make(map[uuid.UUID][]model.RecommendationRule),
	}
}

func (c *AnalysisCache) refresh(repo *repository.HealthRepository) {
	ctx := context.Background()

	analyses, err := repo.ListAnalysis(ctx)
	if err != nil {
		log.Printf("cache refresh analyses: %v", err)
		return
	}
	criteria, err := repo.ListCriteria(ctx, nil)
	if err != nil {
		log.Printf("cache refresh criteria: %v", err)
		return
	}
	rules, err := repo.GetAllRecommendationRules(ctx)
	if err != nil {
		log.Printf("cache refresh rules: %v", err)
		return
	}

	am := make(map[uuid.UUID]model.Analysis, len(analyses))
	for _, a := range analyses {
		am[a.ID] = a
	}
	cm := make(map[uuid.UUID]model.Criterion, len(criteria))
	for _, c := range criteria {
		cm[c.ID] = c
	}
	rbm := make(map[uuid.UUID][]model.RecommendationRule)
	for _, r := range rules {
		rbm[r.CriterionID] = append(rbm[r.CriterionID], r)
	}

	c.mu.Lock()
	c.analyses = analyses
	c.criteria = criteria
	c.analysisMap = am
	c.criterionMap = cm
	c.rulesByCriterion = rbm
	c.mu.Unlock()
}

// RunRefreshLoop starts background loop that refreshes cache every minute.
func (c *AnalysisCache) RunRefreshLoop(ctx context.Context, repo *repository.HealthRepository) {
	c.refresh(repo) // initial load
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(repo)
		}
	}
}

func (c *AnalysisCache) GetAnalyses() []model.Analysis {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Analysis, len(c.analyses))
	copy(out, c.analyses)
	return out
}

func (c *AnalysisCache) GetCriteria() []model.Criterion {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Criterion, len(c.criteria))
	copy(out, c.criteria)
	return out
}

func (c *AnalysisCache) GetAnalysis(id uuid.UUID) (model.Analysis, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	a, ok := c.analysisMap[id]
	return a, ok
}

func (c *AnalysisCache) GetCriterion(id uuid.UUID) (model.Criterion, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cr, ok := c.criterionMap[id]
	return cr, ok
}

func (c *AnalysisCache) GetRulesForCriterion(id uuid.UUID) []model.RecommendationRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rulesByCriterion[id]
}

// GetCriteriaForAnalysis returns criteria belonging to an analysis.
func (c *AnalysisCache) GetCriteriaForAnalysis(analysisID uuid.UUID) []model.Criterion {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []model.Criterion
	for _, cr := range c.criteria {
		if cr.AnalysisID == analysisID {
			out = append(out, cr)
		}
	}
	return out
}
