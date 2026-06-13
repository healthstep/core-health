package service

import (
	"strconv"

	"github.com/helthtech/core-health/internal/model"
)

// DashboardCriterionStatus returns the status (empty / ok / critical) and a
// baseline recommendation. The recommendation is intentionally empty for
// healthy ("ok") values; concrete advice for empty/out-of-range values is
// layered on top from the recommendations table.
func DashboardCriterionStatus(c model.Criterion, value string) (status string, recommendation string) {
	switch c.InputType {
	case "check":
		if value == "" {
			return "empty", "Нет отметки — выполните и отметьте показатель."
		}
		if value == "1" {
			return "ok", ""
		}
		return "empty", "Нет отметки — выполните и отметьте показатель."

	case "boolean":
		if value == "" {
			return "empty", "Нет данных — укажите результат."
		}
		// "good" answer is "Да" (1) by default, or "Нет" (0) when negative is normal.
		goodValue := "1"
		if c.NegativeIsNormal {
			goodValue = "0"
		}
		if value == goodValue {
			return "ok", ""
		}
		return "critical", "Результат вне нормы — стоит разобраться в причине."

	case "numeric":
		if value == "" {
			return "empty", "Нет данных — внесите показатель."
		}
		if c.MinValue == nil && c.MaxValue == nil {
			return "ok", ""
		}
		numVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "ok", ""
		}
		inNormal := (c.MinValue == nil || numVal >= *c.MinValue) &&
			(c.MaxValue == nil || numVal <= *c.MaxValue)
		if inNormal {
			return "ok", ""
		}
		return "critical", "Значение вне нормы."

	default:
		if value == "" {
			return "empty", "Нет данных — внесите показатель."
		}
		return "ok", ""
	}
}
