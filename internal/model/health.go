package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Criterion is a single health metric.
// BlockedBy: "", "level_1", "level_2", or "criteria_<uuid>"
// Sex: "male", "female", or "" for all users
// InputType: "numeric" or "check"
// Lifetime: days after entry before expiry; 0 = no expiry
type Criterion struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string    `gorm:"type:text;not null"`
	Level     int       `gorm:"type:int;not null;default:1"`
	Sex       string    `gorm:"type:text;not null;default:''"`
	BlockedBy string    `gorm:"type:text;not null;default:''"`
	InputType string    `gorm:"type:text;not null;default:'numeric'"`
	Lifetime  int       `gorm:"type:int;not null;default:0"`
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
