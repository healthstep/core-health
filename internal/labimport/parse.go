package labimport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/helthtech/core-health/internal/gigachat"
	"github.com/helthtech/core-health/internal/model"
	"github.com/helthtech/core-health/internal/obs"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	maxTextRunes = 100_000
	parseLocale  = "ru"
	defaultSex   = "любой"

	typeNumeric = "numeric"
	typeBoolean = "boolean"
	typeCheck   = "check"

	valYes = "1"
	valNo  = "0"

	dateISO = "2006-01-02"

	criteriaHeader   = "Разрешённые критерии (используй ТОЛЬКО эти id, ничего другого):\n"
	criteriaLineFmt  = "- id=%q название=%q тип=%q"
	criteriaRangeFmt = " норма=%.6g..%.6g"
	criteriaLineEnd  = "\n"

	userPromptHeaderFmt = "Текст документа (медицинский / лабораторный бланк):\n-----\n%s\n-----\nПол пользователя: %s. Локаль: %s\n\n"

	userPromptRules = `
Верни JSON-объект строго такой структуры:
{"document_date":"","items":[{"criterion_id":"","value":"","measured_at":""}],"note":""}

Правила:
- "document_date": дата сдачи анализов в формате ISO YYYY-MM-DD (ищи в шапке/подвале); пусто, если не нашёл.
- "measured_at": дата конкретного показателя, если указана отдельно; иначе скопируй document_date; пусто, если неизвестно.
- "value": всегда строка. Числовое значение — с точкой как десятичным разделителем ("5.2"). Для boolean/check — только "0" или "1".
- Включай КАЖДЫЙ показатель из текста, который разумно соответствует критерию из списка, даже при умеренной уверенности.
- НЕ придумывай criterion_id — только id из списка выше.
- Если ни один показатель не подходит — верни "items": [].
- Верни ТОЛЬКО JSON, без markdown и без пояснений.`

	systemPrompt = `Ты — парсер медицинских лабораторных бланков для российского приложения о здоровье. Извлеки из текста ВСЕ лабораторные показатели и сопоставь их с предоставленным списком критериев по их criterion_id, используя медицинские знания.

Бланк может быть плохо отформатирован: колонки таблицы съехали, значение и название показателя на разных строках или разделены пробелами. Используй смысловой и позиционный контекст, чтобы сопоставить значение с названием.

Учитывай русские и латинские названия и аббревиатуры:
Лейкоциты=WBC, Эритроциты=RBC, Гемоглобин=HGB/Hb, Гематокрит=HCT/Ht, Тромбоциты=PLT,
Нейтрофилы=NEU/NEUT, Лимфоциты=LYM/LYMPH, Моноциты=MON/MONO, Эозинофилы=EOS, Базофилы=BAS/BASO,
MCV=средний объём эритроцита, MCH=среднее содержание Hb, MCHC=концентрация Hb, RDW=ширина распределения эритроцитов,
СОЭ=ESR, Глюкоза=GLU/Glucose, HbA1c=гликированный гемоглобин, Холестерин общий=CHOL/Total Cholesterol,
ЛПВП=HDL, ЛПНП=LDL, Триглицериды=TG/TRIG, Коэффициент атерогенности=КА/IA,
Креатинин=CREA/Creatinine, Мочевина=Urea/BUN, Мочевая кислота=UA/Uric acid, Билирубин=BILI/Bilirubin,
АЛТ=ALT/ALAT, АСТ=AST/ASAT, ТТГ=TSH, Ферритин=Ferritin.

Отвечай ТОЛЬКО валидным JSON-объектом, без markdown, без текста до или после.`
)

var dateLayouts = []string{dateISO, "02.01.2006", "02/01/2006", "02-01-2006", "2006/01/02"}

type ParsedResult struct {
	CriterionID string
	Value       string
	MeasuredAt  string
}

type Parser struct {
	ai *gigachat.Client
	db *gorm.DB
}

func NewParser(db *gorm.DB) *Parser {
	return &Parser{ai: gigachat.NewFromEnv(), db: db}
}

type llmOut struct {
	DocumentDate string    `json:"document_date,omitempty"`
	Items        []llmItem `json:"items"`
	Note         string    `json:"note,omitempty"`
}

type llmItem struct {
	CriterionID string `json:"criterion_id"`
	Value       string `json:"value"`
	MeasuredAt  string `json:"measured_at,omitempty"`
}

type criterionSent struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Type string   `json:"type"`
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
}

func (p *Parser) Parse(ctx context.Context, text, userSex string, criteria []model.Criterion) ([]ParsedResult, string, error) {
	if n := utf8.RuneCountInString(text); n > maxTextRunes {
		text = string([]rune(text)[:maxTextRunes])
	}

	allowed := make(map[string]model.Criterion, len(criteria))
	sent := make([]criterionSent, 0, len(criteria))
	b := strings.Builder{}
	b.WriteString(criteriaHeader)
	for _, c := range criteria {
		it := strings.ToLower(c.InputType)
		if it != typeNumeric && it != typeBoolean && it != typeCheck {
			continue
		}
		id := c.ID.String()
		allowed[id] = c
		b.WriteString(fmt.Sprintf(criteriaLineFmt, id, c.Name, c.InputType))
		if c.MinValue != nil || c.MaxValue != nil {
			b.WriteString(fmt.Sprintf(criteriaRangeFmt, deref(c.MinValue), deref(c.MaxValue)))
		}
		b.WriteString(criteriaLineEnd)
		sent = append(sent, criterionSent{ID: id, Name: c.Name, Type: c.InputType, Min: c.MinValue, Max: c.MaxValue})
	}
	if len(allowed) == 0 {
		return nil, "", nil
	}

	sex := userSex
	if sex == "" {
		sex = defaultSex
	}
	userPrompt := fmt.Sprintf(userPromptHeaderFmt, text, sex, parseLocale) + b.String() + userPromptRules

	rec := model.GigachatParseLog{
		Model:        p.ai.Model(),
		UserSex:      userSex,
		DocumentText: text,
		CriteriaSent: toJSON(sent),
	}

	raw, err := p.ai.ChatJSON(ctx, systemPrompt, userPrompt)
	rec.RawResponse = raw
	if err != nil {
		rec.Error = err.Error()
		p.writeLog(ctx, &rec)
		return nil, "", err
	}

	var parsed llmOut
	raw = strings.TrimSpace(raw)
	if errJ := json.Unmarshal([]byte(raw), &parsed); errJ != nil {
		if i, j := strings.Index(raw, "{"), strings.LastIndex(raw, "}"); i >= 0 && j > i {
			_ = json.Unmarshal([]byte(raw[i:j+1]), &parsed)
		}
		if len(parsed.Items) == 0 {
			rec.Error = fmt.Sprintf("llm json: %v", errJ)
			p.writeLog(ctx, &rec)
			return nil, "", fmt.Errorf("llm json: %w", errJ)
		}
	}

	docDate := normalizeDate(strings.TrimSpace(parsed.DocumentDate))
	results := make([]ParsedResult, 0, len(parsed.Items))
	for _, it := range parsed.Items {
		id := strings.TrimSpace(it.CriterionID)
		c, ok := allowed[id]
		if id == "" || !ok {
			continue
		}
		val := normalizeValue(c.InputType, strings.TrimSpace(it.Value))
		if val == "" {
			continue
		}
		measuredAt := normalizeDate(strings.TrimSpace(it.MeasuredAt))
		if measuredAt == "" {
			measuredAt = docDate
		}
		results = append(results, ParsedResult{CriterionID: id, Value: val, MeasuredAt: measuredAt})
	}

	rec.ParsedResult = toJSON(results)
	p.writeLog(ctx, &rec)
	return results, parsed.Note, nil
}

func (p *Parser) writeLog(ctx context.Context, rec *model.GigachatParseLog) {
	if p.db == nil {
		return
	}
	if err := p.db.WithContext(ctx).Create(rec).Error; err != nil {
		obs.BG("labimport").Error(err, "gigachat parse log insert failed")
	}
}

func normalizeValue(inputType, v string) string {
	if v == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(inputType)) {
	case typeBoolean, typeCheck:
		switch strings.ToLower(v) {
		case valYes, "да", "true", "yes":
			return valYes
		case valNo, "нет", "false", "no":
			return valNo
		}
		return v
	default:
		return v
	}
}

func normalizeDate(s string) string {
	if s == "" {
		return ""
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(dateISO)
		}
	}
	return s
}

func deref(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func toJSON(v any) datatypes.JSON {
	b, err := json.Marshal(v)
	if err != nil {
		return datatypes.JSON([]byte("null"))
	}
	return datatypes.JSON(b)
}
