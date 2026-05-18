package service

import "github.com/helthtech/core-health/internal/model"

func CriterionMatchesSex(c model.Criterion, userSex string) bool {
	if c.Sex == "" {
		return true
	}
	if userSex == "" {
		return true
	}
	return c.Sex == userSex
}
