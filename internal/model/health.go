package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Analysis groups related criteria.
// BlockedBy: "", "level_1", "level_2", or "criteria_<uuid>"
// Sex: "male", "female", or "" for all users
// Lifetime: days after entry before expiry; 0 = no expiry
type Analysis struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string    `gorm:"type:text;not null"`
	Lifetime  int       `gorm:"type:int;not null;default:0"`
	Sex       string    `gorm:"type:text;not null;default:''"`
	BlockedBy string    `gorm:"type:text;not null;default:''"`
	CreatedAt time.Time
}

func (Analysis) TableName() string { return "analysis" }

// Criterion is a single health metric belonging to an analysis.
type Criterion struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name       string    `gorm:"type:text;not null"`
	AnalysisID uuid.UUID `gorm:"type:uuid;not null;index"`
	// Level: 1=required, 2=advanced, 3=longevity (live to 100)
	Level     int       `gorm:"type:int;not null;default:1"`
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

// RecommendationRule defines a range → recommendation for a criterion.
// When MinValue == nil && MaxValue == nil → "no data" recommendation.
// Sex filtering is now done at the Analysis level, not here.
type RecommendationRule struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CriterionID    uuid.UUID `gorm:"type:uuid;not null;index"`
	MinValue       *float64  `gorm:"type:decimal"`
	MaxValue       *float64  `gorm:"type:decimal"`
	Recommendation string    `gorm:"type:text;not null"`
	Severity       string    `gorm:"type:text;not null"` // ok, warning, critical
}

func (RecommendationRule) TableName() string { return "recommendation_rules" }

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
