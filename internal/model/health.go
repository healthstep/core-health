package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type HealthCriterion struct {
	ID                      uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Code                    string    `gorm:"type:text;uniqueIndex;not null"`
	Name                    string    `gorm:"type:text;not null"`
	Description             string    `gorm:"type:text"`
	ValueType               string    `gorm:"type:text;not null"` // numeric, boolean, completion
	Unit                    string    `gorm:"type:text"`
	InputModes              string    `gorm:"type:text"` // comma-separated: document,manual,mark_done
	RecurrenceIntervalDays  int       `gorm:"type:int"`
	IsActive                bool      `gorm:"type:bool;default:true"`
	CreatedAt               time.Time
}

func (HealthCriterion) TableName() string { return "health_criteria" }

type LabTest struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Code        string    `gorm:"type:text;uniqueIndex;not null"`
	Name        string    `gorm:"type:text;not null"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time
}

func (LabTest) TableName() string { return "lab_tests" }

type LabTestCriterion struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	LabTestID         uuid.UUID `gorm:"type:uuid;not null;index"`
	HealthCriterionID uuid.UUID `gorm:"type:uuid;not null;index"`
}

func (LabTestCriterion) TableName() string { return "lab_test_criteria" }

type UserCriterionEvent struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID            uuid.UUID  `gorm:"type:uuid;not null;index:idx_events_user_criterion,priority:1"`
	HealthCriterionID *uuid.UUID `gorm:"type:uuid;index:idx_events_user_criterion,priority:2"`
	LabTestID         *uuid.UUID `gorm:"type:uuid"`
	EventType         string     `gorm:"type:text;not null"` // file_upload, numeric_entry, boolean_entry, marked_done
	NumericValue      *float64   `gorm:"type:decimal"`
	BooleanValue      string     `gorm:"type:text"` // yes, no, unknown
	DocumentID        *uuid.UUID `gorm:"type:uuid"`
	Note              string     `gorm:"type:text"`
	OccurredAt        time.Time  `gorm:"type:timestamptz;index:idx_events_user_criterion,priority:3,sort:desc"`
	RecordedAt        time.Time  `gorm:"type:timestamptz"`
	Source            string     `gorm:"type:text;not null"` // telegram, max, web
}

func (UserCriterionEvent) TableName() string { return "user_criterion_events" }

func (e *UserCriterionEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.RecordedAt.IsZero() {
		e.RecordedAt = time.Now()
	}
	return nil
}

type Document struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	StorageKey string    `gorm:"type:text;not null"`
	MimeType   string    `gorm:"type:text"`
	ByteSize   int64     `gorm:"type:bigint"`
	SHA256     string    `gorm:"type:text"`
	CreatedAt  time.Time
}

func (Document) TableName() string { return "documents" }

type NumericRecommendationRule struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	HealthCriterionID uuid.UUID `gorm:"type:uuid;not null;index"`
	MinValue          *float64  `gorm:"type:decimal"`
	MaxValue          *float64  `gorm:"type:decimal"`
	Sex               string    `gorm:"type:text"` // male, female, or empty for all
	Recommendation    string    `gorm:"type:text;not null"`
	Severity          string    `gorm:"type:text;not null"` // ok, warning, risk
}

func (NumericRecommendationRule) TableName() string { return "numeric_recommendation_rules" }

type BooleanRecommendationRule struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	HealthCriterionID uuid.UUID `gorm:"type:uuid;not null;index"`
	TriggerValue      string    `gorm:"type:text;not null"` // yes, no, unknown
	Recommendation    string    `gorm:"type:text;not null"`
	Severity          string    `gorm:"type:text;not null"`
}

func (BooleanRecommendationRule) TableName() string { return "boolean_recommendation_rules" }

type UserCriterionState struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID            uuid.UUID  `gorm:"type:uuid;not null;index"`
	HealthCriterionID uuid.UUID  `gorm:"type:uuid;not null;index"`
	LastEventAt       *time.Time `gorm:"type:timestamptz"`
	LastValue         string     `gorm:"type:text"`
	Status            string     `gorm:"type:text"` // ok, overdue, missing, warning
	EscalationStep    int        `gorm:"type:int;default:0"`
	NextReminderAt    *time.Time `gorm:"type:timestamptz"`
	UpdatedAt         time.Time
}

func (UserCriterionState) TableName() string { return "user_criterion_state" }

type NotificationLog struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID           uuid.UUID `gorm:"type:uuid;not null;index"`
	Channel          string    `gorm:"type:text;not null"` // telegram, max
	NotificationType string    `gorm:"type:text;not null"` // reminder, pressure, risk
	TemplateCode     string    `gorm:"type:text"`
	PayloadSummary   string    `gorm:"type:jsonb"`
	DedupeKey        *string   `gorm:"type:text;uniqueIndex"`
	SentAt           time.Time `gorm:"type:timestamptz"`
	DeliveryStatus   string    `gorm:"type:text;not null;default:'sent'"` // sent, failed
}

func (NotificationLog) TableName() string { return "notification_log" }
