package seed

import (
	"github.com/google/uuid"
	"github.com/helthtech/core-health/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Run(db *gorm.DB) error {
	// --- Groups ---
	gBlood := uid("b0000001-0000-0000-0000-000000000001")
	gBiochem := uid("b0000001-0000-0000-0000-000000000002")
	gPressure := uid("b0000001-0000-0000-0000-000000000003")
	gActivity := uid("b0000001-0000-0000-0000-000000000004")
	gVisits := uid("b0000001-0000-0000-0000-000000000005")
	gInstrumental := uid("b0000001-0000-0000-0000-000000000006")
	gFemale := uid("b0000001-0000-0000-0000-000000000007")
	gVaccinations := uid("b0000001-0000-0000-0000-000000000008")

	groups := []model.CriterionGroup{
		{ID: gBlood, Name: "Кровь", SortOrder: 1},
		{ID: gBiochem, Name: "Биохимия", SortOrder: 2},
		{ID: gPressure, Name: "Давление", SortOrder: 3},
		{ID: gActivity, Name: "Активность", SortOrder: 4},
		{ID: gVisits, Name: "Визиты к врачу", SortOrder: 5},
		{ID: gInstrumental, Name: "Инструментальные", SortOrder: 6},
		{ID: gFemale, Name: "Женское здоровье", SortOrder: 7},
		{ID: gVaccinations, Name: "Вакцинация", SortOrder: 8},
	}
	for _, g := range groups {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&g).Error; err != nil {
			return err
		}
	}

	// Level 1 = required, Level 2 = advanced, Level 3 = longevity
	// InputType: "numeric", "check", or "boolean"
	// BlockedBy: "", "level_1", "level_2", "criteria_<uuid>"
	// Sex: "", "male", "female"
	// Lifetime: days (0 = no expiry)
	criteria := []model.Criterion{
		// --- Blood ---
		{ID: uid("a0000001-0000-0000-0000-000000000001"), GroupID: &gBlood, Name: "Гемоглобин", Level: 1, InputType: "numeric", Lifetime: 365, SortOrder: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000003"), GroupID: &gBlood, Name: "Лейкоциты", Level: 1, InputType: "numeric", Lifetime: 365, SortOrder: 2},
		{ID: uid("a0000001-0000-0000-0000-000000000004"), GroupID: &gBlood, Name: "Тромбоциты", Level: 2, InputType: "numeric", Lifetime: 365, SortOrder: 3},
		// --- Biochemistry ---
		{ID: uid("a0000001-0000-0000-0000-000000000002"), GroupID: &gBiochem, Name: "Глюкоза", Level: 1, InputType: "numeric", Lifetime: 365, SortOrder: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000005"), GroupID: &gBiochem, Name: "Холестерин", Level: 1, InputType: "numeric", Lifetime: 365, SortOrder: 2},
		{ID: uid("a0000001-0000-0000-0000-000000000017"), GroupID: &gBiochem, Name: "Липидный профиль", Level: 3, InputType: "numeric", Lifetime: 365, SortOrder: 3},
		// --- Blood pressure ---
		{ID: uid("a0000001-0000-0000-0000-000000000006"), GroupID: &gPressure, Name: "Давление систолическое", Level: 1, InputType: "numeric", Lifetime: 30, SortOrder: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000007"), GroupID: &gPressure, Name: "Давление диастолическое", Level: 1, InputType: "numeric", Lifetime: 30, SortOrder: 2},
		// --- Vision ---
		{ID: uid("a0000001-0000-0000-0000-000000000008"), GroupID: &gInstrumental, Name: "Острота зрения", Level: 2, InputType: "numeric", Lifetime: 365, SortOrder: 1},
		// --- Activity ---
		{ID: uid("a0000001-0000-0000-0000-000000000009"), GroupID: &gActivity, Name: "Шаги в неделю", Level: 1, InputType: "numeric", Lifetime: 7, SortOrder: 1},
		// --- Preventive visits ---
		{ID: uid("a0000001-0000-0000-0000-000000000010"), GroupID: &gVisits, Name: "Стоматолог", Level: 1, InputType: "check", Lifetime: 180, SortOrder: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000011"), GroupID: &gVisits, Name: "Терапевт", Level: 1, InputType: "check", Lifetime: 365, SortOrder: 2},
		{ID: uid("a0000001-0000-0000-0000-000000000015"), GroupID: &gVisits, Name: "Прививка", Level: 1, InputType: "check", Lifetime: 365, SortOrder: 3},
		// --- Instrumental ---
		{ID: uid("a0000001-0000-0000-0000-000000000012"), GroupID: &gInstrumental, Name: "УЗИ брюшной полости", Level: 2, InputType: "check", Lifetime: 365, SortOrder: 2},
		{ID: uid("a0000001-0000-0000-0000-000000000013"), GroupID: &gInstrumental, Name: "УЗИ щитовидной железы", Level: 2, InputType: "check", Lifetime: 365, SortOrder: 3},
		{ID: uid("a0000001-0000-0000-0000-000000000014"), GroupID: &gVisits, Name: "Флюорография", Level: 1, InputType: "check", Lifetime: 365, SortOrder: 4},
		{ID: uid("a0000001-0000-0000-0000-000000000018"), GroupID: &gInstrumental, Name: "Анализ мочи", Level: 2, InputType: "check", Lifetime: 365, SortOrder: 4},
		// --- Female health ---
		{ID: uid("a0000001-0000-0000-0000-000000000016"), GroupID: &gFemale, Name: "Последняя менструация (дни назад)", Level: 1, InputType: "numeric", Lifetime: 90, Sex: "female", BlockedBy: "level_1", SortOrder: 1},
		// --- Vaccinations ---
		{ID: uid("a0000001-0000-0000-0000-000000000019"), GroupID: &gVaccinations, Name: "Вакцина от гриппа (год)", Level: 1, InputType: "check", Lifetime: 365, BlockedBy: "level_1", SortOrder: 1},
		{ID: uid("a0000001-0000-0000-0000-000000000020"), GroupID: &gVaccinations, Name: "COVID-19 вакцина (серия)", Level: 1, InputType: "check", Lifetime: 365, BlockedBy: "criteria_a0000001-0000-0000-0000-000000000019", SortOrder: 2},
		{ID: uid("a0000001-0000-0000-0000-000000000021"), GroupID: &gVaccinations, Name: "Гепатит B вакцина (серия)", Level: 1, InputType: "check", Lifetime: 365, BlockedBy: "criteria_a0000001-0000-0000-0000-000000000020", SortOrder: 3},
	}
	for _, c := range criteria {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&c).Error; err != nil {
			return err
		}
	}

	// Recommendation rules (status evaluation for dashboard).
	rules := []model.RecommendationRule{
		// Гемоглобин
		{ID: uid("d0000001-0000-0000-0000-000000000001"), CriterionID: criteria[0].ID, MinValue: nil, MaxValue: nil, Recommendation: "Рекомендуем сдать общий анализ крови и внести показатель гемоглобина.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000002"), CriterionID: criteria[0].ID, MinValue: pf(120), MaxValue: pf(175), Recommendation: "Гемоглобин в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000004"), CriterionID: criteria[0].ID, MinValue: nil, MaxValue: pf(119), Recommendation: "Гемоглобин ниже нормы — рекомендуется консультация терапевта.", Severity: "critical"},
		{ID: uid("d0000001-0000-0000-0000-000000000026"), CriterionID: criteria[0].ID, MinValue: pf(175), MaxValue: nil, Recommendation: "Гемоглобин выше нормы — проконсультируйтесь с врачом.", Severity: "warning"},
		// Лейкоциты
		{ID: uid("d0000001-0000-0000-0000-000000000005"), CriterionID: criteria[1].ID, MinValue: nil, MaxValue: nil, Recommendation: "Рекомендуем сдать общий анализ крови и внести показатель лейкоцитов.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000006"), CriterionID: criteria[1].ID, MinValue: pf(4), MaxValue: pf(9), Recommendation: "Лейкоциты в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000007"), CriterionID: criteria[1].ID, MinValue: nil, MaxValue: pf(3.9), Recommendation: "Лейкоциты понижены. Проконсультируйтесь с врачом.", Severity: "critical"},
		{ID: uid("d0000001-0000-0000-0000-000000000008"), CriterionID: criteria[1].ID, MinValue: pf(9.1), MaxValue: nil, Recommendation: "Лейкоциты повышены — возможен воспалительный процесс.", Severity: "critical"},
		// Глюкоза
		{ID: uid("d0000001-0000-0000-0000-000000000009"), CriterionID: criteria[3].ID, MinValue: nil, MaxValue: nil, Recommendation: "Рекомендуем сдать биохимию крови и внести показатель глюкозы.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000010"), CriterionID: criteria[3].ID, MinValue: pf(3.3), MaxValue: pf(5.5), Recommendation: "Глюкоза в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000011"), CriterionID: criteria[3].ID, MinValue: pf(5.6), MaxValue: nil, Recommendation: "Глюкоза повышена. Рекомендуется консультация эндокринолога.", Severity: "critical"},
		// Холестерин
		{ID: uid("d0000001-0000-0000-0000-000000000012"), CriterionID: criteria[4].ID, MinValue: nil, MaxValue: nil, Recommendation: "Рекомендуем сдать биохимию крови и внести показатель холестерина.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000013"), CriterionID: criteria[4].ID, MinValue: pf(0), MaxValue: pf(5.2), Recommendation: "Холестерин в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000014"), CriterionID: criteria[4].ID, MinValue: pf(5.2), MaxValue: nil, Recommendation: "Холестерин повышен — рекомендуется диета и консультация кардиолога.", Severity: "warning"},
		// Давление систолическое
		{ID: uid("d0000001-0000-0000-0000-000000000015"), CriterionID: criteria[6].ID, MinValue: nil, MaxValue: nil, Recommendation: "Измерьте артериальное давление и внесите систолическое значение.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000016"), CriterionID: criteria[6].ID, MinValue: pf(90), MaxValue: pf(130), Recommendation: "Систолическое давление в норме.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000017"), CriterionID: criteria[6].ID, MinValue: pf(130), MaxValue: nil, Recommendation: "Систолическое давление повышено — следите за давлением и проконсультируйтесь с врачом.", Severity: "warning"},
		// Шаги в неделю
		{ID: uid("d0000001-0000-0000-0000-000000000018"), CriterionID: criteria[9].ID, MinValue: nil, MaxValue: nil, Recommendation: "Добавьте данные о своей физической активности — количество шагов в неделю.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000019"), CriterionID: criteria[9].ID, MinValue: pf(70000), MaxValue: nil, Recommendation: "Отличная физическая активность! Продолжайте в том же духе.", Severity: "ok"},
		{ID: uid("d0000001-0000-0000-0000-000000000020"), CriterionID: criteria[9].ID, MinValue: pf(35000), MaxValue: pf(69999), Recommendation: "Умеренная активность. Старайтесь ходить больше!", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000021"), CriterionID: criteria[9].ID, MinValue: nil, MaxValue: pf(34999), Recommendation: "Низкая физическая активность. Рекомендуется не менее 10 000 шагов в день.", Severity: "critical"},
		// Стоматолог
		{ID: uid("d0000001-0000-0000-0000-000000000022"), CriterionID: criteria[10].ID, MinValue: nil, MaxValue: nil, Recommendation: "Посетите стоматолога — профилактика дважды в год снижает риск серьёзных заболеваний.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000023"), CriterionID: criteria[10].ID, MinValue: pf(1), MaxValue: nil, Recommendation: "Стоматолог посещён.", Severity: "ok"},
		// Флюорография
		{ID: uid("d0000001-0000-0000-0000-000000000024"), CriterionID: criteria[15].ID, MinValue: nil, MaxValue: nil, Recommendation: "Пройдите флюорографию — ежегодное обследование лёгких.", Severity: "warning"},
		{ID: uid("d0000001-0000-0000-0000-000000000025"), CriterionID: criteria[15].ID, MinValue: pf(1), MaxValue: nil, Recommendation: "Флюорография пройдена.", Severity: "ok"},
		// Последняя менструация
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

	// New recommendations (notification/auction system).
	js := func(texts []string) datatypes.JSONType[[]string] {
		return datatypes.NewJSONType(texts)
	}
	recommendations := []model.Recommendation{
		// --- Reminders (no data) ---
		{
			ID: uid("e0000001-0000-0000-0000-000000000001"), CriterionID: criteria[0].ID,
			Type: "reminder", Title: "Сдайте анализ на гемоглобин", BaseWeight: 6,
			Texts: js([]string{
				"Напомним: вы ещё не внесли показатель гемоглобина. Сдайте общий анализ крови и добавьте результат.",
				"Гемоглобин — важный маркер здоровья. Пора узнать свой показатель!",
				"Не знаете свой гемоглобин? Самое время сдать ОАК.",
			}),
		},
		{
			ID: uid("e0000001-0000-0000-0000-000000000002"), CriterionID: criteria[3].ID,
			Type: "reminder", Title: "Узнайте уровень глюкозы", BaseWeight: 6,
			Texts: js([]string{
				"Вы ещё не внесли уровень глюкозы. Сдайте биохимию крови и добавьте результат.",
				"Контроль сахара крови — основа профилактики диабета. Внесите данные!",
			}),
		},
		{
			ID: uid("e0000001-0000-0000-0000-000000000003"), CriterionID: criteria[9].ID,
			Type: "reminder", Title: "Начните считать шаги", BaseWeight: 5,
			Texts: js([]string{
				"Вы ещё не указали количество шагов за неделю. Установите шагомер и начните отслеживать активность!",
				"Ходьба — лучшее лекарство. Добавьте данные о своих шагах за неделю.",
			}),
		},
		{
			ID: uid("e0000001-0000-0000-0000-000000000004"), CriterionID: criteria[10].ID,
			Type: "reminder", Title: "Запишитесь к стоматологу", BaseWeight: 5,
			Texts: js([]string{
				"Вы ещё не отметили визит к стоматологу. Профилактика раз в полгода — ключ к здоровью зубов.",
				"Когда вы последний раз были у стоматолога? Запишитесь на профилактический осмотр.",
			}),
		},
		{
			ID: uid("e0000001-0000-0000-0000-000000000005"), CriterionID: criteria[6].ID,
			Type: "reminder", Title: "Измерьте артериальное давление", BaseWeight: 6,
			Texts: js([]string{
				"Вы ещё не вносили показатели давления. Измерьте и добавьте данные — это займёт минуту.",
				"Давление — важный показатель работы сердца. Измерьте его сегодня!",
			}),
		},
		// --- Recommendations (lifestyle, nutrition) ---
		{
			ID: uid("e0000001-0000-0000-0000-000000000010"), CriterionID: criteria[0].ID,
			Type: "recommendation", Title: "Питание для повышения гемоглобина", BaseWeight: 4,
			MinValue: nil, MaxValue: pf(119),
			Texts: js([]string{
				"Сегодня попробуйте съесть гречку или красное мясо — они богаты железом и помогут повысить гемоглобин.",
				"Добавьте в рацион шпинат, чечевицу или говяжью печень — отличные источники железа.",
				"Витамин C помогает усваивать железо. Ешьте продукты, богатые железом, вместе с цитрусовыми.",
			}),
		},
		{
			ID: uid("e0000001-0000-0000-0000-000000000011"), CriterionID: criteria[4].ID,
			Type: "recommendation", Title: "Снизьте холестерин питанием", BaseWeight: 4,
			MinValue: pf(5.2), MaxValue: nil,
			Texts: js([]string{
				"Сегодня замените жирное мясо на рыбу или бобовые — это поможет снизить холестерин.",
				"Овсяная каша на завтрак — отличный способ снизить уровень холестерина.",
				"Добавьте орехи и авокадо в рацион: они содержат «хороший» холестерин.",
			}),
		},
		{
			ID: uid("e0000001-0000-0000-0000-000000000012"), CriterionID: criteria[9].ID,
			Type: "recommendation", Title: "Увеличьте физическую активность", BaseWeight: 4,
			MinValue: nil, MaxValue: pf(34999),
			Texts: js([]string{
				"Сегодня пройдите хотя бы 30 минут пешком — это около 3 000 шагов. Маленький шаг к здоровью!",
				"Поднимитесь по лестнице вместо лифта и сделайте небольшую прогулку в обед.",
				"Поставьте цель: добавить 1 000 шагов к вашему обычному маршруту сегодня.",
			}),
		},
		{
			ID: uid("e0000001-0000-0000-0000-000000000013"), CriterionID: criteria[6].ID,
			Type: "recommendation", Title: "Контроль давления: образ жизни", BaseWeight: 4,
			MinValue: pf(130), MaxValue: nil,
			Texts: js([]string{
				"Сократите потребление соли сегодня — замените солёные снеки на свежие овощи.",
				"Медитация 10 минут в день помогает снизить артериальное давление. Попробуйте сегодня.",
				"Физическая активность 30 минут в день снижает давление. Выйдите на прогулку!",
			}),
		},
		// --- Alarms ---
		{
			ID: uid("e0000001-0000-0000-0000-000000000020"), CriterionID: criteria[1].ID,
			Type: "alarm", Title: "Тревога: лейкоциты вне нормы", BaseWeight: 10,
			Texts: js([]string{
				"⚠️ Ваши лейкоциты вне нормы. Это может указывать на воспалительный процесс. Пожалуйста, обратитесь к врачу.",
				"⚠️ Показатель лейкоцитов требует внимания. Проконсультируйтесь с терапевтом.",
			}),
		},
		{
			ID: uid("e0000001-0000-0000-0000-000000000021"), CriterionID: criteria[3].ID,
			Type: "alarm", Title: "Тревога: глюкоза повышена", BaseWeight: 10,
			MinValue: pf(5.6), MaxValue: nil,
			Texts: js([]string{
				"⚠️ Ваш уровень глюкозы повышен. Пожалуйста, проконсультируйтесь с эндокринологом.",
				"⚠️ Высокий сахар крови — повод немедленно обратиться к врачу.",
			}),
		},
	}
	for _, r := range recommendations {
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
