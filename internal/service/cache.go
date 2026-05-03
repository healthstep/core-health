package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"github.com/helthtech/core-health/internal/obs"
	"github.com/helthtech/core-health/internal/repository"
)

// CriteriaCache holds in-memory snapshots of groups, analyses, criteria, and recommendations, refreshed from the database every minute.
type CriteriaCache struct {
	mu              sync.RWMutex
	groups          []model.CriterionGroup
	analyses        []model.Analysis
	analysisByID    map[int64]model.Analysis
	criteria        []model.Criterion
	criterionMap    map[uuid.UUID]model.Criterion
	recommendations []model.Recommendation
	recsByCriterion map[uuid.UUID][]model.Recommendation
}

func NewCriteriaCache() *CriteriaCache {
	return &CriteriaCache{
		criterionMap:    make(map[uuid.UUID]model.Criterion),
		recsByCriterion: make(map[uuid.UUID][]model.Recommendation),
		analysisByID:    make(map[int64]model.Analysis),
	}
}

func (c *CriteriaCache) refresh(repo *repository.HealthRepository) {
	ctx := context.Background()
	log := obs.BG("cache")

	groups, err := repo.ListGroups(ctx)
	if err != nil {
		log.Error(err, "cache: refresh groups")
	}

	analyses, err := repo.ListAnalyses(ctx)
	if err != nil {
		log.Error(err, "cache: refresh analyses")
	}

	criteria, err := repo.ListCriteria(ctx)
	if err != nil {
		log.Error(err, "cache: refresh criteria")
		return
	}
	recs, err := repo.GetAllRecommendations(ctx)
	if err != nil {
		log.Error(err, "cache: refresh recommendations")
	}

	cm := make(map[uuid.UUID]model.Criterion, len(criteria))
	for _, cr := range criteria {
		cm[cr.ID] = cr
	}
	am := make(map[int64]model.Analysis, len(analyses))
	for _, a := range analyses {
		am[a.ID] = a
	}
	recm := make(map[uuid.UUID][]model.Recommendation)
	for _, r := range recs {
		recm[r.CriterionID] = append(recm[r.CriterionID], r)
	}

	c.mu.Lock()
	c.groups = groups
	c.analyses = analyses
	c.analysisByID = am
	c.criteria = criteria
	c.criterionMap = cm
	c.recommendations = recs
	c.recsByCriterion = recm
	c.mu.Unlock()
}

// RunRefreshLoop starts a background loop that refreshes the cache every minute.
func (c *CriteriaCache) RunRefreshLoop(ctx context.Context, repo *repository.HealthRepository) {
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

func (c *CriteriaCache) GetGroups() []model.CriterionGroup {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.CriterionGroup, len(c.groups))
	copy(out, c.groups)
	return out
}

func (c *CriteriaCache) GetCriteria() []model.Criterion {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Criterion, len(c.criteria))
	copy(out, c.criteria)
	return out
}

func (c *CriteriaCache) GetCriterion(id uuid.UUID) (model.Criterion, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cr, ok := c.criterionMap[id]
	return cr, ok
}

func (c *CriteriaCache) GetAnalyses() []model.Analysis {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Analysis, len(c.analyses))
	copy(out, c.analyses)
	return out
}

func (c *CriteriaCache) GetAnalysis(id int64) (model.Analysis, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	a, ok := c.analysisByID[id]
	return a, ok
}

func (c *CriteriaCache) AnalysisInstructionForCriterion(cr model.Criterion) string {
	if cr.AnalysisID == nil {
		return ""
	}
	c.mu.RLock()
	a, ok := c.analysisByID[*cr.AnalysisID]
	c.mu.RUnlock()
	if !ok {
		return ""
	}
	return a.Instruction
}

func (c *CriteriaCache) AnalysisNameForCriterion(cr model.Criterion) string {
	if cr.AnalysisID == nil {
		return ""
	}
	c.mu.RLock()
	a, ok := c.analysisByID[*cr.AnalysisID]
	c.mu.RUnlock()
	if !ok {
		return ""
	}
	return a.Name
}

func (c *CriteriaCache) GetAllRecommendations() []model.Recommendation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Recommendation, len(c.recommendations))
	copy(out, c.recommendations)
	return out
}

func (c *CriteriaCache) GetRecsForCriterion(id uuid.UUID) []model.Recommendation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.recsByCriterion[id]
}
