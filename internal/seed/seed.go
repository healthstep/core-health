package seed

import (
	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Run(db *gorm.DB) error {
	criteria := []model.HealthCriterion{
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000001"), Code: "hemoglobin", Name: "Гемоглобин", ValueType: "numeric", Unit: "г/л", InputModes: "document,manual", RecurrenceIntervalDays: 180, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000002"), Code: "glucose", Name: "Глюкоза", ValueType: "numeric", Unit: "ммоль/л", InputModes: "document,manual", RecurrenceIntervalDays: 365, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000003"), Code: "leukocytes", Name: "Лейкоциты", ValueType: "numeric", Unit: "× 10⁹/л", InputModes: "document,manual", RecurrenceIntervalDays: 180, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000004"), Code: "platelets", Name: "Тромбоциты", ValueType: "numeric", Unit: "× 10⁹/л", InputModes: "document,manual", RecurrenceIntervalDays: 180, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000005"), Code: "cholesterol", Name: "Холестерин", ValueType: "numeric", Unit: "ммоль/л", InputModes: "document,manual", RecurrenceIntervalDays: 365, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000006"), Code: "blood_pressure_sys", Name: "Давление (систолическое)", ValueType: "numeric", Unit: "мм рт. ст.", InputModes: "manual", RecurrenceIntervalDays: 90, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000007"), Code: "blood_pressure_dia", Name: "Давление (диастолическое)", ValueType: "numeric", Unit: "мм рт. ст.", InputModes: "manual", RecurrenceIntervalDays: 90, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000008"), Code: "visual_acuity", Name: "Острота зрения", ValueType: "numeric", Unit: "", InputModes: "manual", RecurrenceIntervalDays: 365, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000009"), Code: "weekly_steps", Name: "Шаги / неделю", ValueType: "numeric", Unit: "шагов", InputModes: "manual", RecurrenceIntervalDays: 7, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000010"), Code: "dentist_visit", Name: "Стоматолог", ValueType: "completion", Unit: "", InputModes: "mark_done", RecurrenceIntervalDays: 180, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000011"), Code: "therapist_visit", Name: "Терапевт", ValueType: "completion", Unit: "", InputModes: "mark_done", RecurrenceIntervalDays: 365, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000012"), Code: "ultrasound_abdominal", Name: "УЗИ брюшной полости", ValueType: "completion", Unit: "", InputModes: "mark_done,document", RecurrenceIntervalDays: 365, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000013"), Code: "ultrasound_thyroid", Name: "УЗИ щитовидной железы", ValueType: "completion", Unit: "", InputModes: "mark_done,document", RecurrenceIntervalDays: 365, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000014"), Code: "fluorography", Name: "Флюорография", ValueType: "completion", Unit: "", InputModes: "mark_done,document", RecurrenceIntervalDays: 365, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000015"), Code: "vaccination", Name: "Прививка", ValueType: "completion", Unit: "", InputModes: "mark_done", RecurrenceIntervalDays: 365, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000016"), Code: "body_temperature", Name: "Температура тела", ValueType: "numeric", Unit: "°C", InputModes: "manual", RecurrenceIntervalDays: 0, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000017"), Code: "lipid_profile", Name: "Липидный профиль", ValueType: "numeric", Unit: "", InputModes: "document,manual", RecurrenceIntervalDays: 365, IsActive: true},
		{ID: uuidFromStr("a0000001-0000-0000-0000-000000000018"), Code: "urinalysis", Name: "Анализ мочи", ValueType: "completion", Unit: "", InputModes: "document,mark_done", RecurrenceIntervalDays: 365, IsActive: true},
	}

	for _, c := range criteria {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&c).Error; err != nil {
			return err
		}
	}

	labTests := []model.LabTest{
		{ID: uuidFromStr("b0000001-0000-0000-0000-000000000001"), Code: "cbc", Name: "Общий анализ крови", Description: "Общий клинический анализ крови"},
		{ID: uuidFromStr("b0000001-0000-0000-0000-000000000002"), Code: "biochemistry", Name: "Биохимия крови", Description: "Биохимический анализ крови"},
	}
	for _, t := range labTests {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&t).Error; err != nil {
			return err
		}
	}

	links := []model.LabTestCriterion{
		{ID: uuidFromStr("c0000001-0000-0000-0000-000000000001"), LabTestID: labTests[0].ID, HealthCriterionID: criteria[0].ID},
		{ID: uuidFromStr("c0000001-0000-0000-0000-000000000002"), LabTestID: labTests[0].ID, HealthCriterionID: criteria[2].ID},
		{ID: uuidFromStr("c0000001-0000-0000-0000-000000000003"), LabTestID: labTests[0].ID, HealthCriterionID: criteria[3].ID},
		{ID: uuidFromStr("c0000001-0000-0000-0000-000000000004"), LabTestID: labTests[1].ID, HealthCriterionID: criteria[1].ID},
		{ID: uuidFromStr("c0000001-0000-0000-0000-000000000005"), LabTestID: labTests[1].ID, HealthCriterionID: criteria[4].ID},
	}
	for _, l := range links {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&l).Error; err != nil {
			return err
		}
	}

	rules := []model.NumericRecommendationRule{
		{ID: uuidFromStr("d0000001-0000-0000-0000-000000000001"), HealthCriterionID: criteria[0].ID, MinValue: pf(130), MaxValue: pf(175), Sex: "male", Recommendation: "Норма", Severity: "ok"},
		{ID: uuidFromStr("d0000001-0000-0000-0000-000000000002"), HealthCriterionID: criteria[0].ID, MinValue: pf(120), MaxValue: pf(160), Sex: "female", Recommendation: "Норма", Severity: "ok"},
		{ID: uuidFromStr("d0000001-0000-0000-0000-000000000003"), HealthCriterionID: criteria[0].ID, MinValue: nil, MaxValue: pf(120), Sex: "", Recommendation: "Гемоглобин ниже нормы. Обратитесь к терапевту.", Severity: "warning"},
		{ID: uuidFromStr("d0000001-0000-0000-0000-000000000004"), HealthCriterionID: criteria[1].ID, MinValue: pf(3.3), MaxValue: pf(5.5), Sex: "", Recommendation: "Норма", Severity: "ok"},
		{ID: uuidFromStr("d0000001-0000-0000-0000-000000000005"), HealthCriterionID: criteria[4].ID, MinValue: pf(0), MaxValue: pf(5.2), Sex: "", Recommendation: "Норма", Severity: "ok"},
		{ID: uuidFromStr("d0000001-0000-0000-0000-000000000006"), HealthCriterionID: criteria[4].ID, MinValue: pf(5.2), MaxValue: nil, Sex: "", Recommendation: "Холестерин повышен. Обратитесь к врачу.", Severity: "warning"},
	}
	for _, r := range rules {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&r).Error; err != nil {
			return err
		}
	}

	return nil
}

func uuidFromStr(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

func pf(v float64) *float64 { return &v }
