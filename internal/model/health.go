package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type CriterionGroup struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string    `gorm:"type:text;not null"`
	SortOrder int       `gorm:"type:int;not null;default:0"`
}

func (CriterionGroup) TableName() string { return "criterion_groups" }

type Analysis struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"type:text;not null"`
	Instruction string `gorm:"type:text;not null;default:''"`
	CreatedAt   time.Time
}

func (Analysis) TableName() string { return "analyses" }

type Criterion struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	GroupID          *uuid.UUID `gorm:"type:uuid;index"`
	AnalysisID       *int64     `gorm:"index"`
	Name             string     `gorm:"type:text;not null"`
	Level            int        `gorm:"type:int;not null;default:1"`
	Sex              string     `gorm:"type:text;not null;default:''"`
	InputType        string     `gorm:"type:text;not null;default:'numeric'"`
	Lifetime         int        `gorm:"type:int;not null;default:0"`
	SortOrder        int        `gorm:"type:int;not null;default:0"`
	MinValue         *float64   `gorm:"type:decimal"`
	MaxValue         *float64   `gorm:"type:decimal"`
	Delta            *float64   `gorm:"type:decimal"`
	Description      string     `gorm:"type:text;not null;default:''"`
	NegativeIsNormal bool       `gorm:"type:boolean;not null;default:false"`
	CreatedAt        time.Time
}

func (Criterion) TableName() string { return "criteria" }

type UserCriterion struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_user_criterion"`
	CriterionID uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_user_criterion"`
	Value       string     `gorm:"type:text"`
	MeasuredAt  *time.Time `gorm:"type:date"`
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

type Recommendation struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CriterionID   uuid.UUID `gorm:"type:uuid;not null;index"`
	Type          string    `gorm:"type:text;not null;default:'recommendation'"`
	Title         string    `gorm:"type:text;not null"`
	BaseWeight    int       `gorm:"type:int;not null;default:1"`
	MinValue      *float64  `gorm:"type:decimal"`
	MaxValue      *float64  `gorm:"type:decimal"`
	CreatedAt     time.Time
	Notifications []RecommendationNotification `gorm:"foreignKey:RecommendationID;constraint:OnDelete:CASCADE"`
}

func (Recommendation) TableName() string { return "recommendations" }

type RecommendationNotification struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	RecommendationID uuid.UUID `gorm:"type:uuid;not null;index"`
	Text             string    `gorm:"type:text;not null"`
	CreatedAt        time.Time
}

func (RecommendationNotification) TableName() string { return "recommendation_notifications" }

func (n *RecommendationNotification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

type WeeklyRecommendation struct {
	ID        uuid.UUID                          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID                          `gorm:"type:uuid;not null;uniqueIndex:idx_user_week"`
	WeekStart time.Time                          `gorm:"type:date;not null;uniqueIndex:idx_user_week"`
	Weights   datatypes.JSONType[map[string]int] `gorm:"type:jsonb;not null"`
	UpdatedAt time.Time
}

func (WeeklyRecommendation) TableName() string { return "weekly_recommendations" }

type NotificationLog struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID           uuid.UUID `gorm:"type:uuid;not null;index"`
	Channel          string    `gorm:"type:text;not null"`
	NotificationType string    `gorm:"type:text;not null"`
	TemplateCode     string    `gorm:"type:text"`
	PayloadSummary   string    `gorm:"type:jsonb"`
	DedupeKey        *string   `gorm:"type:text;uniqueIndex"`
	SentAt           time.Time `gorm:"type:timestamptz"`
	DeliveryStatus   string    `gorm:"type:text;not null;default:'sent'"`
}

func (NotificationLog) TableName() string { return "notification_log" }

type GigachatParseLog struct {
	ID           int64          `gorm:"primaryKey;autoIncrement"`
	CreatedAt    time.Time      `gorm:"type:timestamptz;not null;default:now()"`
	Model        string         `gorm:"type:text"`
	UserSex      string         `gorm:"type:text"`
	DocumentText string         `gorm:"type:text"`
	CriteriaSent datatypes.JSON `gorm:"type:jsonb"`
	RawResponse  string         `gorm:"type:text"`
	ParsedResult datatypes.JSON `gorm:"type:jsonb"`
	Error        string         `gorm:"type:text"`
}

func (GigachatParseLog) TableName() string { return "gigachat_parse_log" }
