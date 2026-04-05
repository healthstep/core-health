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

// CriteriaCache holds in-memory snapshots of criteria and recommendation rules,
// refreshed from the database every minute.
type CriteriaCache struct {
	mu               sync.RWMutex
	criteria         []model.Criterion
	criterionMap     map[uuid.UUID]model.Criterion
	rulesByCriterion map[uuid.UUID][]model.RecommendationRule
}

func NewCriteriaCache() *CriteriaCache {
	return &CriteriaCache{
		criterionMap:     make(map[uuid.UUID]model.Criterion),
		rulesByCriterion: make(map[uuid.UUID][]model.RecommendationRule),
	}
}

func (c *CriteriaCache) refresh(repo *repository.HealthRepository) {
	ctx := context.Background()

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

	cm := make(map[uuid.UUID]model.Criterion, len(criteria))
	for _, cr := range criteria {
		cm[cr.ID] = cr
	}
	rbm := make(map[uuid.UUID][]model.RecommendationRule)
	for _, r := range rules {
		rbm[r.CriterionID] = append(rbm[r.CriterionID], r)
	}

	c.mu.Lock()
	c.criteria = criteria
	c.criterionMap = cm
	c.rulesByCriterion = rbm
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
