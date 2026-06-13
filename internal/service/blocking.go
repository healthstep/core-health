package service

import (
	"github.com/helthtech/core-health/internal/model"
)

func CriterionMatchesSex(c model.Criterion, userSex string) bool {
	if c.Sex == "" {
		return true
	}
	if userSex == "" {
		return true
	}
	return c.Sex == userSex
}

func CriterionVisibleForUser(c model.Criterion, uc UserContext) bool {
	if !CriterionMatchesSex(c, uc.Sex) {
		return false
	}
	// Level 1 = mandatory (shown to everyone), level 2 = optional (advanced users only).
	if c.Level > 1 && !uc.Advanced {
		return false
	}
	return true
}
