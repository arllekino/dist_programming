package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"uuid"

	goredis "github.com/redis/go-redis/v9"

	"session/internal/session/domain"
)

const (
	sessionKeyPrefix      = "session:"
	userSessionsKeyPrefix = "user_sessions:"
)

func NewSessionRepository(ctx context.Context, client goredis.UniversalClient) domain.SessionRepository {
	return &sessionRepository{
		ctx:    ctx,
		client: client,
	}
}

type sessionRepository struct {
	ctx    context.Context
	client goredis.UniversalClient
}

type record struct {
	UserID            string    `json:"user_id"`
	CreatedAt         time.Time `json:"created_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	AbsoluteExpiresAt time.Time `json:"absolute_expires_at"`
}

func (repo *sessionRepository) Create(s domain.Session) error {
	value, err := encode(s)
	if err != nil {
		return err
	}
	userKey := userSessionsKey(s.UserID)
	indexTTL := time.Until(s.AbsoluteExpiresAt)
	if indexTTL <= 0 {
		return fmt.Errorf("session %s is already past its absolute expiration", s.ID)
	}
	_, err = repo.client.TxPipelined(repo.ctx, func(pipe goredis.Pipeliner) error {
		pipe.SetArgs(repo.ctx, sessionKey(s.ID), value, goredis.SetArgs{
			Mode:     "NX",
			ExpireAt: s.ExpiresAt,
		})
		pipe.SAdd(repo.ctx, userKey, s.ID)
		pipe.ExpireNX(repo.ctx, userKey, indexTTL)
		pipe.ExpireGT(repo.ctx, userKey, indexTTL)
		return nil
	})
	return err
}

func (repo *sessionRepository) Find(id string) (domain.Session, error) {
	value, err := repo.client.Get(repo.ctx, sessionKey(id)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	if err != nil {
		return domain.Session{}, err
	}
	return decode(id, value)
}

func (repo *sessionRepository) Update(s domain.Session) error {
	value, err := encode(s)
	if err != nil {
		return err
	}
	err = repo.client.SetArgs(repo.ctx, sessionKey(s.ID), value, goredis.SetArgs{
		Mode:     "XX",
		ExpireAt: s.ExpiresAt,
	}).Err()
	if errors.Is(err, goredis.Nil) {
		return domain.ErrSessionNotFound
	}
	return err
}

func (repo *sessionRepository) Delete(id string) (domain.Session, error) {
	value, err := repo.client.GetDel(repo.ctx, sessionKey(id)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	if err != nil {
		return domain.Session{}, err
	}
	s, err := decode(id, value)
	if err != nil {
		return domain.Session{}, err
	}
	err = repo.client.SRem(repo.ctx, userSessionsKey(s.UserID), id).Err()
	if err != nil {
		return domain.Session{}, err
	}
	return s, nil
}

func (repo *sessionRepository) FindByUser(userID uuid.UUID) ([]domain.Session, error) {
	userKey := userSessionsKey(userID)
	ids, err := repo.client.SMembers(repo.ctx, userKey).Result()
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, sessionKey(id))
	}
	values, err := repo.client.MGet(repo.ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	sessions := make([]domain.Session, 0, len(ids))
	var stale []any
	for i, value := range values {
		encoded, ok := value.(string)
		if !ok {
			stale = append(stale, ids[i])
			continue
		}
		s, err := decode(ids[i], []byte(encoded))
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	if len(stale) > 0 {
		err = repo.client.SRem(repo.ctx, userKey, stale...).Err()
		if err != nil {
			return nil, err
		}
	}
	return sessions, nil
}

func sessionKey(id string) string {
	return sessionKeyPrefix + id
}

func userSessionsKey(userID uuid.UUID) string {
	return userSessionsKeyPrefix + userID.String()
}

func encode(s domain.Session) ([]byte, error) {
	return json.Marshal(record{
		UserID:            s.UserID.String(),
		CreatedAt:         s.CreatedAt,
		ExpiresAt:         s.ExpiresAt,
		AbsoluteExpiresAt: s.AbsoluteExpiresAt,
	})
}

func decode(id string, value []byte) (domain.Session, error) {
	var rec record
	err := json.Unmarshal(value, &rec)
	if err != nil {
		return domain.Session{}, fmt.Errorf("decode session %s: %w", id, err)
	}
	userID, err := uuid.Parse(rec.UserID)
	if err != nil {
		return domain.Session{}, fmt.Errorf("decode session %s: %w", id, err)
	}
	return domain.Session{
		ID:                id,
		UserID:            userID,
		CreatedAt:         rec.CreatedAt,
		ExpiresAt:         rec.ExpiresAt,
		AbsoluteExpiresAt: rec.AbsoluteExpiresAt,
	}, nil
}
