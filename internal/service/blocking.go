package service

import "github.com/helthtech/core-health/internal/model"

// CriterionMatchesSex returns true if the criterion is available for the given user sex.
// An empty criterion.Sex means available to all.
func CriterionMatchesSex(c model.Criterion, userSex string) bool {
	if c.Sex == "" {
		return true
	}
	if userSex == "" {
		return true
	}
	return c.Sex == userSex
}
