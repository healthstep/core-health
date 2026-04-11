package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CriterionGroup groups criteria for display in bots and dashboard.
type CriterionGroup struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string    `gorm:"type:text;not null"`
	SortOrder int       `gorm:"type:int;not null;default:0"`
}

func (CriterionGroup) TableName() string { return "criterion_groups" }

// Criterion is a single health metric.
// Sex: "male", "female", or "" for all users
// InputType: "numeric", "check", or "boolean"
//   - numeric: user enters a number; MinValue/MaxValue/Delta define normal range and warning zone
//   - check:   user marks whether they have done this (e.g., visited dentist)
//   - boolean: binary result — "1" positive (ok), "0" negative (alarm)
// Lifetime: days after entry before expiry; 0 = no expiry
// GroupID: optional reference to CriterionGroup for UI grouping
// MinValue/MaxValue: normal range bounds (numeric only)
// Delta: non-critical (warning) deviation width from norm boundary
type Criterion struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GroupID   *uuid.UUID `gorm:"type:uuid;index"`
	Name      string     `gorm:"type:text;not null"`
	Level     int        `gorm:"type:int;not null;default:1"`
	Sex       string     `gorm:"type:text;not null;default:''"`
	InputType string     `gorm:"type:text;not null;default:'numeric'"`
	Lifetime  int        `gorm:"type:int;not null;default:0"`
	SortOrder int        `gorm:"type:int;not null;default:0"`
	MinValue  *float64   `gorm:"type:decimal"`
	MaxValue  *float64   `gorm:"type:decimal"`
	Delta     *float64   `gorm:"type:decimal"`
	CreatedAt time.Time
}

func (Criterion) TableName() string { return "criteria" }

// UserCriterion stores the current value of a criterion for a user.
// Supports soft-delete via DeletedAt (GORM convention).
type UserCriterion struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_criterion"`
	CriterionID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_user_criterion"`
	Value       string         `gorm:"type:text"`
	MeasuredAt  *time.Time     `gorm:"type:date"` // date when the test/measurement was actually performed
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (UserCriterion) TableName() string { return "user_criteria" }

func (uc *UserCriterion) BeforeCreate(tx *gorm.DB) error {
	if uc.ID == uuid.Nil {
		uc.ID = uuid.New()
	}
	return nil
}

// Recommendation is the new recommendation entity for the notification/auction system.
//
// Type:
//   - "reminder"              — user has no value for this criterion
//   - "recommendation"        — actionable suggestion (nutrition, lifestyle, etc.)
//   - "alarm"                 — values significantly out of norm (sent separately, not in daily auction)
//   - "expiration_reminder"   — data is about to expire (sent by the expiry scheduler)
//
// Texts: multiple notification text variants; one is picked randomly per send.
// BaseWeight: initial auction weight; higher = more likely to be picked in daily auction.
// Applicability is derived from the linked Criterion's MinValue/MaxValue/Delta fields.
type Recommendation struct {
	ID          uuid.UUID                    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CriterionID uuid.UUID                    `gorm:"type:uuid;not null;index"`
	Type        string                       `gorm:"type:text;not null;default:'recommendation'"`
	Title       string                       `gorm:"type:text;not null"`
	Texts       datatypes.JSONType[[]string] `gorm:"type:jsonb;not null"`
	BaseWeight  int                          `gorm:"type:int;not null;default:1"`
	CreatedAt   time.Time
}

func (Recommendation) TableName() string { return "recommendations" }

// WeeklyRecommendation stores per-user per-week recommendation weights for the daily auction.
// Generated fresh every Monday; weight decreases after each daily send and reaches 0 when spent.
type WeeklyRecommendation struct {
	ID        uuid.UUID                          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID                          `gorm:"type:uuid;not null;uniqueIndex:idx_user_week"`
	WeekStart time.Time                          `gorm:"type:date;not null;uniqueIndex:idx_user_week"`
	Weights   datatypes.JSONType[map[string]int] `gorm:"type:jsonb;not null"`
	UpdatedAt time.Time
}

func (WeeklyRecommendation) TableName() string { return "weekly_recommendations" }

// NotificationLog records sent notifications.
type NotificationLog struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID           uuid.UUID `gorm:"type:uuid;not null;index"`
	Channel          string    `gorm:"type:text;not null"` // telegram, max
	NotificationType string    `gorm:"type:text;not null"`
	TemplateCode     string    `gorm:"type:text"`
	PayloadSummary   string    `gorm:"type:jsonb"`
	DedupeKey        *string   `gorm:"type:text;uniqueIndex"`
	SentAt           time.Time `gorm:"type:timestamptz"`
	DeliveryStatus   string    `gorm:"type:text;not null;default:'sent'"`
}

func (NotificationLog) TableName() string { return "notification_log" }
