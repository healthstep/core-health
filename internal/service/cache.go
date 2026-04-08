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

// CriteriaCache holds in-memory snapshots of groups, criteria, recommendation rules, and
// recommendations, refreshed from the database every minute.
type CriteriaCache struct {
	mu                  sync.RWMutex
	groups              []model.CriterionGroup
	criteria            []model.Criterion
	criterionMap        map[uuid.UUID]model.Criterion
	rulesByCriterion    map[uuid.UUID][]model.RecommendationRule
	recommendations     []model.Recommendation
	recsByCriterion     map[uuid.UUID][]model.Recommendation
}

func NewCriteriaCache() *CriteriaCache {
	return &CriteriaCache{
		criterionMap:     make(map[uuid.UUID]model.Criterion),
		rulesByCriterion: make(map[uuid.UUID][]model.RecommendationRule),
		recsByCriterion:  make(map[uuid.UUID][]model.Recommendation),
	}
}

func (c *CriteriaCache) refresh(repo *repository.HealthRepository) {
	ctx := context.Background()

	groups, err := repo.ListGroups(ctx)
	if err != nil {
		log.Printf("cache refresh groups: %v", err)
	}

	criteria, err := repo.ListCriteria(ctx)
	if err != nil {
		log.Printf("cache refresh criteria: %v", err)
		return
	}
	rules, err := repo.GetAllRecommendationRules(ctx)
	if err != nil {
		log.Printf("cache refresh rules: %v", err)
		return
	}
	recs, err := repo.GetAllRecommendations(ctx)
	if err != nil {
		log.Printf("cache refresh recommendations: %v", err)
	}

	cm := make(map[uuid.UUID]model.Criterion, len(criteria))
	for _, cr := range criteria {
		cm[cr.ID] = cr
	}
	rbm := make(map[uuid.UUID][]model.RecommendationRule)
	for _, r := range rules {
		rbm[r.CriterionID] = append(rbm[r.CriterionID], r)
	}
	recm := make(map[uuid.UUID][]model.Recommendation)
	for _, r := range recs {
		recm[r.CriterionID] = append(recm[r.CriterionID], r)
	}

	c.mu.Lock()
	c.groups = groups
	c.criteria = criteria
	c.criterionMap = cm
	c.rulesByCriterion = rbm
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

func (c *CriteriaCache) GetRulesForCriterion(id uuid.UUID) []model.RecommendationRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rulesByCriterion[id]
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
