package app

import (
	"context"
	"time"
	"uuid"

	"session/internal/session/domain"
)

type RepositoryProvider interface {
	Session(ctx context.Context) domain.SessionRepository
}

type Service struct {
	repos  RepositoryProvider
	policy domain.Policy
	now    func() time.Time
}

func NewService(repos RepositoryProvider, policy domain.Policy, now func() time.Time) *Service {
	return &Service{
		repos:  repos,
		policy: policy,
		now:    now,
	}
}

func (s *Service) CreateSession(ctx context.Context, userID uuid.UUID) (domain.Token, domain.Session, error) {
	return s.sessions(ctx).Create(userID)
}

func (s *Service) ValidateSession(ctx context.Context, token string) (domain.Session, error) {
	return s.sessions(ctx).Validate(token)
}

func (s *Service) RevokeSession(ctx context.Context, token string) (domain.Session, error) {
	return s.sessions(ctx).Revoke(token)
}

func (s *Service) RevokeUserSessions(ctx context.Context, userID uuid.UUID, exceptSessionID string, createdBefore time.Time) (int, error) {
	return s.sessions(ctx).RevokeUserSessions(userID, exceptSessionID, createdBefore)
}

func (s *Service) sessions(ctx context.Context) *domain.SessionService {
	return domain.NewSessionService(s.repos.Session(ctx), s.policy, s.now)
}
