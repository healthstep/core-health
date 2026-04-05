package service

import (
	"strings"

	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
)

// IsAnalysisBlocked returns true if the analysis is not yet available to the user
// given their current filled criteria.
//
// Supported BlockedBy values:
//   - ""         → never blocked
//   - "level_1"  → blocked until all level-1 criteria are filled
//   - "level_2"  → blocked until all level-2 criteria are filled
//   - "criteria_<uuid>" → blocked until that specific criterion is filled
func IsAnalysisBlocked(analysis model.Analysis, allCriteria []model.Criterion, userValues map[uuid.UUID]string) bool {
	switch {
	case analysis.BlockedBy == "":
		return false

	case analysis.BlockedBy == "level_1":
		for _, c := range allCriteria {
			if c.Level == 1 && userValues[c.ID] == "" {
				return true
			}
		}
		return false

	case analysis.BlockedBy == "level_2":
		for _, c := range allCriteria {
			if c.Level == 2 && userValues[c.ID] == "" {
				return true
			}
		}
		return false

	case strings.HasPrefix(analysis.BlockedBy, "criteria_"):
		idStr := strings.TrimPrefix(analysis.BlockedBy, "criteria_")
		id, err := uuid.Parse(idStr)
		if err != nil {
			return false
		}
		return userValues[id] == ""

	default:
		return false
	}
}

// MatchesSex returns true if the analysis is available for the given user sex.
// An empty analysis.Sex means available to all.
func MatchesSex(analysis model.Analysis, userSex string) bool {
	if analysis.Sex == "" {
		return true
	}
	if userSex == "" {
		// unknown sex: show all analyses (safer UX)
		return true
	}
	return analysis.Sex == userSex
}
