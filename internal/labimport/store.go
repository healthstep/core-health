package labimport

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "labimport:pending:"

// Store keeps a JSON blob in Redis for ~1h until the user confirms.
type Store struct {
	rdb *redis.Client
}

func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

// PendingBatch is what we persist before writing to user_criteria.
type PendingBatch struct {
	UserID string         `json:"user_id"`
	Items  []PendingItem  `json:"items"`
}

// PendingItem mirrors what the UI / API need to show the user.
type PendingItem struct {
	CriterionID   string `json:"criterion_id"`
	CriterionName string `json:"criterion_name"`
	Value         string `json:"value"`
	InputType     string `json:"input_type"`
	MeasuredAt    string `json:"measured_at,omitempty"`
	Instruction   string `json:"instruction,omitempty"`
}

func (s *Store) Save(ctx context.Context, pendingID string, b PendingBatch, ttl time.Duration) error {
	if s == nil || s.rdb == nil {
		return fmt.Errorf("redis not configured")
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyPrefix+pendingID, raw, ttl).Err()
}

func (s *Store) Load(ctx context.Context, pendingID string) (*PendingBatch, error) {
	if s == nil || s.rdb == nil {
		return nil, fmt.Errorf("redis not configured")
	}
	raw, err := s.rdb.Get(ctx, keyPrefix+pendingID).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("not found or expired")
	}
	if err != nil {
		return nil, err
	}
	var b PendingBatch
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Store) Delete(ctx context.Context, pendingID string) error {
	if s == nil || s.rdb == nil {
		return nil
	}
	return s.rdb.Del(ctx, keyPrefix+pendingID).Err()
}
