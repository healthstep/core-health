package seed

import (
	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Run(db *gorm.DB) error {
	// Level 1 = required, Level 2 = advanced, Level 3 = longevity
	// InputType: "numeric" or "check"
	// BlockedBy: "", "level_1", "level_2", "criteria_<uuid>"
	// Sex: "", "male", "female"
	// Lifetime: days (0 = no expiry)
	criteria := []model.Criterion{
		// --- Blood: required ---
		{ID: uid("a0000001-0000-0000-0000-000000000001"), Name: "Гемоглобин", Level: 1, InputType: "numeric", Lifetime: 365},
		{ID: uid("a0000001-0000-0000-0000-000000000003"), Name: "Лейкоциты", Level: 1, InputType: "numeric", Lifetime: 365},
		{ID: uid("a0000001-0000-0000-0000-000000000004"), Name: "Тромбоциты", Level: 2, InputType: "numeric", Lifetime: 365},
		// --- Biochemistry ---
		{ID: uid("a0000001-0000-0000-0000-000000000002"), Name: "Глюкоза", Level: 1, InputType: "numeric", Lifetime: 365},
		{ID: uid("a0000001-0000-0000-0000-000000000005"), Name: "Холестерин", Level: 1, InputType: "numeric", Lifetime: 365},
		{ID: uid("a0000001-0000-0000-0000-000000000017"), Name: "Липидный профиль", Level: 3, InputType: "numeric", Lifetime: 365},
		// --- Blood pressure ---
		{ID: uid("a0000001-0000-0000-0000-000000000006"), Name: "Давление систолическое", Level: 1, InputType: "numeric", Lifetime: 30},
		{ID: uid("a0000001-0000-0000-0000-000000000007"), Name: "Давление диастолическое", Level: 1, InputType: "numeric", Lifetime: 30},
		// --- Vision ---
		{ID: uid("a0000001-0000-0000-0000-000000000008"), Name: "Острота зрения", Level: 2, InputType: "numeric", Lifetime: 365},
		// --- Activity ---
		{ID: uid("a0000001-0000-0000-0000-000000000009"), Name: "Шаги в неделю", Level: 1, InputType: "numeric", Lifetime: 7},
		// --- Preventive visits (check-type) ---
		{ID: uid("a0000001-0000-0000-0000-000000000010"), Name: "Стоматолог", Level: 1, InputType: "check", Lifetime: 180},
		{ID: uid("a0000001-0000-0000-0000-000000000011"), Name: "Терапевт", Level: 1, InputType: "check", Lifetime: 365},
		{ID: uid("a0000001-0000-0000-0000-000000000015"), Name: "Прививка", Level: 1, InputType: "check", Lifetime: 365},
		// --- Instrumental (check-type) ---
		{ID: uid("a0000001-0000-0000-0000-000000000012"), Name: "УЗИ брюшной полости", Level: 2, InputType: "check", Lifetime: 365},
		{ID: uid("a0000001-0000-0000-0000-000000000013"), Name: "УЗИ щитовидной железы", Level: 2, InputType: "check", Lifetime: 365},
		{ID: uid("a0000001-0000-0000-0000-000000000014"), Name: "Флюорография", Level: 1, InputType: "check", Lifetime: 365},
		{ID: uid("a0000001-0000-0000-0000-000000000018"), Name: "Анализ мочи", Level: 2, InputType: "check", Lifetime: 365},
		// --- Female health (sex-restricted, blocked until level_1 complete) ---
		{ID: uid("a0000001-0000-0000-0000-000000000016"), Name: "Последняя менструация (дни назад)", Level: 1, InputType: "numeric", Lifetime: 90, Sex: "female", BlockedBy: "level_1"},
		// --- Vaccinations (sequential blocking) ---
		{ID: uid("a0000001-0000-0000-0000-000000000019"), Name: "Вакцина от гриппа (год)", Level: 1, InputType: "check", Lifetime: 365, BlockedBy: "level_1"},
		{ID: uid("a0000001-0000-0000-0000-000000000020"), Name: "COVID-19 вакцина (серия)", Level: 1, InputType: "check", Lifetime: 365, BlockedBy: "criteria_a0000001-0000-0000-0000-000000000019"},
		{ID: uid("a0000001-0000-0000-0000-000000000021"), Name: "Гепатит B вакцина (серия)", Level: 1, InputType: "check", Lifetime: 365, BlockedBy: "criteria_a0000001-0000-0000-0000-000000000020"},
	}
	for _, c := range criteria {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&c).Error; err != nil {
			return err
		}
	}

	// Recommendation rules. nil/nil min/max = "no data" recommendation.
	rules := []model.RecommendationRule{
		// Гемоглобин (criteria[0])
		{ID: uid("d0000001-0000-0000-0000-000000000001"), CriterionID: criteria[0].ID, MinValue: nil, MaxValue: nil, Recommendation: "Рекомендуем сдать общий анализ крови и внести показатель гемоглобина.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000002"), CriterionID: criteria[0].ID, MinValue: pf(120), MaxValue: pf(175), Recommendation: "Гемоглобин в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000004"), CriterionID: criteria[0].ID, MinValue: nil, MaxValue: pf(119), Recommendation: "Гемоглобин ниже нормы — рекомендуется консультация терапевта.", Severity: "critical"},
		{ID: uid("d0000001-0000-0000-0000-000000000026"), CriterionID: criteria[0].ID, MinValue: pf(175), MaxValue: nil, Recommendation: "Гемоглобин выше нормы — проконсультируйтесь с врачом.", Severity: "warning"},

		// Лейкоциты (criteria[1])
		{ID: uid("d0000001-0000-0000-0000-000000000005"), CriterionID: criteria[1].ID, MinValue: nil, MaxValue: nil, Recommendation: "Рекомендуем сдать общий анализ крови и внести показатель лейкоцитов.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000006"), CriterionID: criteria[1].ID, MinValue: pf(4), MaxValue: pf(9), Recommendation: "Лейкоциты в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000007"), CriterionID: criteria[1].ID, MinValue: nil, MaxValue: pf(3.9), Recommendation: "Лейкоциты понижены. Проконсультируйтесь с врачом.", Severity: "critical"},
		{ID: uid("d0000001-0000-0000-0000-000000000008"), CriterionID: criteria[1].ID, MinValue: pf(9.1), MaxValue: nil, Recommendation: "Лейкоциты повышены — возможен воспалительный процесс.", Severity: "critical"},

		// Глюкоза (criteria[3])
		{ID: uid("d0000001-0000-0000-0000-000000000009"), CriterionID: criteria[3].ID, MinValue: nil, MaxValue: nil, Recommendation: "Рекомендуем сдать биохимию крови и внести показатель глюкозы.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000010"), CriterionID: criteria[3].ID, MinValue: pf(3.3), MaxValue: pf(5.5), Recommendation: "Глюкоза в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000011"), CriterionID: criteria[3].ID, MinValue: pf(5.6), MaxValue: nil, Recommendation: "Глюкоза повышена. Рекомендуется консультация эндокринолога.", Severity: "critical"},

		// Холестерин (criteria[4])
		{ID: uid("d0000001-0000-0000-0000-000000000012"), CriterionID: criteria[4].ID, MinValue: nil, MaxValue: nil, Recommendation: "Рекомендуем сдать биохимию крови и внести показатель холестерина.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000013"), CriterionID: criteria[4].ID, MinValue: pf(0), MaxValue: pf(5.2), Recommendation: "Холестерин в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000014"), CriterionID: criteria[4].ID, MinValue: pf(5.2), MaxValue: nil, Recommendation: "Холестерин повышен — рекомендуется диета и консультация кардиолога.", Severity: "warning"},

		// Давление систолическое (criteria[6])
		{ID: uid("d0000001-0000-0000-0000-000000000015"), CriterionID: criteria[6].ID, MinValue: nil, MaxValue: nil, Recommendation: "Измерьте артериальное давление и внесите систолическое значение.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000016"), CriterionID: criteria[6].ID, MinValue: pf(90), MaxValue: pf(130), Recommendation: "Систолическое давление в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000017"), CriterionID: criteria[6].ID, MinValue: pf(130), MaxValue: nil, Recommendation: "Систолическое давление повышено — следите за давлением и проконсультируйтесь с врачом.", Severity: "warning"},

		// Шаги в неделю (criteria[9])
		{ID: uid("d0000001-0000-0000-0000-000000000018"), CriterionID: criteria[9].ID, MinValue: nil, MaxValue: nil, Recommendation: "Добавьте данные о своей физической активности — количество шагов в неделю.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000019"), CriterionID: criteria[9].ID, MinValue: pf(70000), MaxValue: nil, Recommendation: "Отличная физическая активность! Продолжайте в том же духе.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000020"), CriterionID: criteria[9].ID, MinValue: pf(35000), MaxValue: pf(69999), Recommendation: "Умеренная активность. Старайтесь ходить больше!", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000021"), CriterionID: criteria[9].ID, MinValue: nil, MaxValue: pf(34999), Recommendation: "Низкая физическая активность. Рекомендуется не менее 10 000 шагов в день.", Severity: "critical"},

		// Стоматолог (criteria[10])
		{ID: uid("d0000001-0000-0000-0000-000000000022"), CriterionID: criteria[10].ID, MinValue: nil, MaxValue: nil, Recommendation: "Посетите стоматолога — профилактика дважды в год снижает риск серьёзных заболеваний.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000023"), CriterionID: criteria[10].ID, MinValue: pf(1), MaxValue: nil, Recommendation: "Стоматолог посещён.", Severity: "ok"},

		// Флюорография (criteria[14])
		{ID: uid("d0000001-0000-0000-0000-000000000024"), CriterionID: criteria[14].ID, MinValue: nil, MaxValue: nil, Recommendation: "Пройдите флюорографию — ежегодное обследование лёгких.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000025"), CriterionID: criteria[14].ID, MinValue: pf(1), MaxValue: nil, Recommendation: "Флюорография пройдена.", Severity: "ok"},

		// Последняя менструация (criteria[17])
		{ID: uid("d0000001-0000-0000-0000-000000000027"), CriterionID: criteria[17].ID, MinValue: nil, MaxValue: nil, Recommendation: "Укажите, сколько дней назад началась последняя менструация.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000028"), CriterionID: criteria[17].ID, MinValue: pf(21), MaxValue: pf(35), Recommendation: "Цикл в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000029"), CriterionID: criteria[17].ID, MinValue: nil, MaxValue: pf(20), Recommendation: "Цикл короткий — рекомендуется консультация гинеколога.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000030"), CriterionID: criteria[17].ID, MinValue: pf(36), MaxValue: nil, Recommendation: "Цикл длинный — рекомендуется консультация гинеколога.", Severity: "warning"},
	}
	for _, r := range rules {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&r).Error; err != nil {
			return err
		}
	}

	return nil
}

func uid(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}

func pf(v float64) *float64 { return &v }
