package service

import (
	"strconv"

	"github.com/helthtech/core-health/internal/model"
)

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
		inNormal := (c.MinValue == nil || numVal >= *c.MinValue) &&
			(c.MaxValue == nil || numVal <= *c.MaxValue)
		if inNormal {
			return "ok", "Показатель в норме."
		}
		return "critical", "Значение вне нормы — рекомендуется проконсультироваться с врачом."

	default:
		if value == "" {
			return "empty", "Нет данных — внесите показатель."
		}
		return "ok", ""
	}
}
