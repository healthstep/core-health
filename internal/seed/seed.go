package seed

import (
	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Run(db *gorm.DB) error {
	// Lifetime in days (0 = no expiry). BlockedBy: "", "level_1", "level_2", "criteria_<uuid>"
	analyses := []model.Analysis{
		{ID: uid("b0000001-0000-0000-0000-000000000001"), Name: "Общий анализ крови", Lifetime: 365},
		{ID: uid("b0000001-0000-0000-0000-000000000002"), Name: "Биохимия крови", Lifetime: 365},
		{ID: uid("b0000001-0000-0000-0000-000000000003"), Name: "Давление", Lifetime: 30},
		{ID: uid("b0000001-0000-0000-0000-000000000004"), Name: "Зрение", Lifetime: 365},
		{ID: uid("b0000001-0000-0000-0000-000000000005"), Name: "Активность", Lifetime: 7},
		{ID: uid("b0000001-0000-0000-0000-000000000006"), Name: "Профилактические визиты", Lifetime: 180},
		{ID: uid("b0000001-0000-0000-0000-000000000007"), Name: "Инструментальные обследования", Lifetime: 365},
		// Женское здоровье — только для женщин, доступно после заполнения уровня 1
		{ID: uid("b0000001-0000-0000-0000-000000000008"), Name: "Женское здоровье", Lifetime: 90, Sex: "female", BlockedBy: "level_1"},
		// Вакцинации — последовательно разблокируемые
		{ID: uid("b0000001-0000-0000-0000-000000000009"), Name: "Вакцина 1 (Грипп)", Lifetime: 365, BlockedBy: "level_1"},
		{ID: uid("b0000001-0000-0000-0000-000000000010"), Name: "Вакцина 2 (COVID-19)", Lifetime: 365, BlockedBy: "criteria_a0000001-0000-0000-0000-000000000019"},
		{ID: uid("b0000001-0000-0000-0000-000000000011"), Name: "Вакцина 3 (Гепатит B)", Lifetime: 365, BlockedBy: "criteria_a0000001-0000-0000-0000-000000000020"},
	}
	for _, a := range analyses {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&a).Error; err != nil {
			return err
		}
	}

	// Level 1 = required, Level 2 = advanced, Level 3 = longevity
	criteria := []model.Criterion{
		// Общий анализ крови — level 1
		{ID: uid("a0000001-0000-0000-0000-000000000001"), Name: "Гемоглобин", AnalysisID: analyses[0].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000003"), Name: "Лейкоциты", AnalysisID: analyses[0].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000004"), Name: "Тромбоциты", AnalysisID: analyses[0].ID, Level: 2},

		// Биохимия крови — level 1
		{ID: uid("a0000001-0000-0000-0000-000000000002"), Name: "Глюкоза", AnalysisID: analyses[1].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000005"), Name: "Холестерин", AnalysisID: analyses[1].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000017"), Name: "Липидный профиль", AnalysisID: analyses[1].ID, Level: 3},

		// Давление — level 1
		{ID: uid("a0000001-0000-0000-0000-000000000006"), Name: "Давление систолическое", AnalysisID: analyses[2].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000007"), Name: "Давление диастолическое", AnalysisID: analyses[2].ID, Level: 1},

		// Зрение — level 2
		{ID: uid("a0000001-0000-0000-0000-000000000008"), Name: "Острота зрения", AnalysisID: analyses[3].ID, Level: 2},

		// Активность — level 1
		{ID: uid("a0000001-0000-0000-0000-000000000009"), Name: "Шаги в неделю", AnalysisID: analyses[4].ID, Level: 1},

		// Профилактические визиты — level 1
		{ID: uid("a0000001-0000-0000-0000-000000000010"), Name: "Стоматолог", AnalysisID: analyses[5].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000011"), Name: "Терапевт", AnalysisID: analyses[5].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000015"), Name: "Прививка", AnalysisID: analyses[5].ID, Level: 1},

		// Инструментальные обследования — level 2
		{ID: uid("a0000001-0000-0000-0000-000000000012"), Name: "УЗИ брюшной полости", AnalysisID: analyses[6].ID, Level: 2},
		{ID: uid("a0000001-0000-0000-0000-000000000013"), Name: "УЗИ щитовидной железы", AnalysisID: analyses[6].ID, Level: 2},
		{ID: uid("a0000001-0000-0000-0000-000000000014"), Name: "Флюорография", AnalysisID: analyses[6].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000018"), Name: "Анализ мочи", AnalysisID: analyses[6].ID, Level: 2},

		// Женское здоровье (analyses[7])
		{ID: uid("a0000001-0000-0000-0000-000000000016"), Name: "Последняя менструация (дни назад)", AnalysisID: analyses[7].ID, Level: 1},

		// Вакцины — used as blocking criteria for subsequent vaccines
		{ID: uid("a0000001-0000-0000-0000-000000000019"), Name: "Вакцина от гриппа (год)", AnalysisID: analyses[8].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000020"), Name: "COVID-19 вакцина (серия)", AnalysisID: analyses[9].ID, Level: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000021"), Name: "Гепатит B вакцина (серия)", AnalysisID: analyses[10].ID, Level: 1},
	}
	for _, c := range criteria {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&c).Error; err != nil {
			return err
		}
	}

	// Recommendation rules. nil/nil min/max = "no data" recommendation.
	// Sex filtering is handled at the Analysis level, not here.
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

		// Флюорография (criteria[16])
		{ID: uid("d0000001-0000-0000-0000-000000000024"), CriterionID: criteria[16].ID, MinValue: nil, MaxValue: nil, Recommendation: "Пройдите флюорографию — ежегодное обследование лёгких.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000025"), CriterionID: criteria[16].ID, MinValue: pf(1), MaxValue: nil, Recommendation: "Флюорография пройдена.", Severity: "ok"},

		// Последняя менструация (criteria[17], женское здоровье)
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
