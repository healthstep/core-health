package service

import (
	"strings"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
)

// IsCriterionBlocked returns true if the criterion is not yet available to the user
// given their current filled criteria.
//
// Supported BlockedBy values:
//   - ""               → never blocked
//   - "level_1"        → blocked until all level-1 criteria are filled
//   - "level_2"        → blocked until all level-2 criteria are filled
//   - "criteria_<uuid>" → blocked until that specific criterion is filled
func IsCriterionBlocked(c model.Criterion, allCriteria []model.Criterion, userValues map[uuid.UUID]string) bool {
	switch {
	case c.BlockedBy == "":
		return false

	case c.BlockedBy == "level_1":
		for _, other := range allCriteria {
			if other.Level == 1 && other.ID != c.ID && userValues[other.ID] == "" {
				return true
			}
		}
		return false

	case c.BlockedBy == "level_2":
		for _, other := range allCriteria {
			if other.Level == 2 && other.ID != c.ID && userValues[other.ID] == "" {
				return true
			}
		}
		return false

	case strings.HasPrefix(c.BlockedBy, "criteria_"):
		idStr := strings.TrimPrefix(c.BlockedBy, "criteria_")
		id, err := uuid.Parse(idStr)
		if err != nil {
			return false
		}
		return userValues[id] == ""

	default:
		return false
	}
}

// CriterionMatchesSex returns true if the criterion is available for the given user sex.
// An empty criterion.Sex means available to all.
func CriterionMatchesSex(c model.Criterion, userSex string) bool {
	if c.Sex == "" {
		return true
	}
	if userSex == "" {
		// unknown sex: show all (safer UX)
		return true
	}
	return c.Sex == userSex
}
