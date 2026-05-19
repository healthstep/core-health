package service

import (
	"math"
	"time"

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

func ageFromBirthDate(s string) int {
	if s == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0
	}
	now := time.Now()
	years := now.Year() - t.Year()
	if now.Month() < t.Month() || (now.Month() == t.Month() && now.Day() < t.Day()) {
		years--
	}
	if years < 0 {
		return 0
	}
	return years
}

func criterionActivated(c model.Criterion, userAge int) bool {
	if c.Lifetime != 0 {
		return true
	}
	overrides := c.LifetimeOverrides.Data()
	if len(overrides) == 0 {
		return true
	}
	minAge := math.MaxInt32
	for age := range overrides {
		if age < minAge {
			minAge = age
		}
	}
	return userAge >= minAge
}

func CriterionVisibleForUser(c model.Criterion, uc UserContext) bool {
	if !CriterionMatchesSex(c, uc.Sex) {
		return false
	}
	userAge := ageFromBirthDate(uc.BirthDate)
	if !criterionActivated(c, userAge) {
		return false
	}
	if c.Level > 1 && !uc.Advanced {
		return false
	}
	return true
}
