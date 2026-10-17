package app

import (
	"context"
	"errors"
	"time"
	"uuid"
)

var (
	ErrUnauthenticated       = errors.New("unauthenticated")
	ErrDependencyUnavailable = errors.New("dependency unavailable")
	ErrInvalidProfile        = errors.New("invalid profile")
)

type Session struct {
	Token     string
	ID        string
	UserID    uuid.UUID
	ExpiresAt time.Time
}

type SessionClient interface {
	Create(ctx context.Context, userID uuid.UUID) (Session, error)
	Validate(ctx context.Context, token string) (Session, error)
	Revoke(ctx context.Context, token string) (Session, error)
	RevokeUserSessions(ctx context.Context, userID uuid.UUID, exceptSessionID string, createdBefore time.Time) error
}

type Profile struct {
	FirstName string
	LastName  string
	Email     string
}

type IdentityClient interface {
	CreateUser(ctx context.Context, userID uuid.UUID, profile Profile) error
}
