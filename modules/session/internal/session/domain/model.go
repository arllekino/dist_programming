package domain

import (
	"time"
	"uuid"
)

type Policy struct {
	IdleTTL     time.Duration
	AbsoluteTTL time.Duration
}

func NewPolicy(idleTTL, absoluteTTL time.Duration) (Policy, error) {
	if idleTTL <= 0 || absoluteTTL < idleTTL {
		return Policy{}, ErrInvalidPolicy
	}
	return Policy{
		IdleTTL:     idleTTL,
		AbsoluteTTL: absoluteTTL,
	}, nil
}

type Session struct {
	ID                string
	UserID            uuid.UUID
	CreatedAt         time.Time
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
}

func NewSession(id string, userID uuid.UUID, now time.Time, policy Policy) (Session, error) {
	if userID == uuid.Nil() {
		return Session{}, ErrInvalidUserID
	}
	now = now.UTC()
	return Session{
		ID:                id,
		UserID:            userID,
		CreatedAt:         now,
		ExpiresAt:         now.Add(policy.IdleTTL),
		AbsoluteExpiresAt: now.Add(policy.AbsoluteTTL),
	}, nil
}

func (s Session) IsExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

func (s *Session) Touch(now time.Time, policy Policy) bool {
	if s.IsExpired(now) || s.ExpiresAt.Sub(now) >= policy.IdleTTL/2 {
		return false
	}
	expiresAt := now.UTC().Add(policy.IdleTTL)
	if expiresAt.After(s.AbsoluteExpiresAt) {
		expiresAt = s.AbsoluteExpiresAt
	}
	if !expiresAt.After(s.ExpiresAt) {
		return false
	}
	s.ExpiresAt = expiresAt
	return true
}

type SessionRepository interface {
	Create(Session) error
	Find(id string) (Session, error)
	Update(Session) error
	Delete(id string) (Session, error)
	FindByUser(userID uuid.UUID) ([]Session, error)
}
