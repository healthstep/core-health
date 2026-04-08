package service

import (
	"math"
	"strconv"

	"github.com/helthtech/core-health/internal/model"
)

// DashboardCriterionStatus derives dashboard status and a short recommendation from the criterion definition
// (min/max/delta for numeric; same warning-band logic as isRecommendationApplicable).
func DashboardCriterionStatus(c model.Criterion, value string) (status string, recommendation string) {
	switch c.InputType {
	case "check":
		if value == "" {
			return "empty", "Нет отметки — выполните и отметьте показатель."
		}
		if value == "1" {
			return "ok", "Выполнено."
		}
		return "empty", ""

	case "boolean":
		if value == "" {
			return "empty", "Нет данных — укажите результат."
		}
		if value == "1" {
			return "ok", "Результат в норме."
		}
		if value == "0" {
			return "critical", "Отрицательный результат — обратитесь к врачу."
		}
		return "empty", ""

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
		delta := 0.0
		if c.Delta != nil {
			delta = *c.Delta
		}
		inNormal := (c.MinValue == nil || numVal >= *c.MinValue) &&
			(c.MaxValue == nil || numVal <= *c.MaxValue)
		if inNormal {
			return "ok", "Показатель в норме."
		}
		warnLow := math.Inf(-1)
		warnHigh := math.Inf(1)
		if c.MinValue != nil {
			warnLow = *c.MinValue - delta
		}
		if c.MaxValue != nil {
			warnHigh = *c.MaxValue + delta
		}
		if numVal >= warnLow && numVal <= warnHigh {
			return "warning", "Слегка вне нормы — при необходимости проконсультируйтесь с врачом."
		}
		return "critical", "Сильное отклонение от нормы — рекомендуется обратиться к врачу."

	default:
		if value == "" {
			return "empty", "Нет данных — внесите показатель."
		}
		return "ok", ""
	}
}
