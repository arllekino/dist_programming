package domain

import (
	"errors"
	"time"
	"uuid"
)

type SessionService struct {
	repo   SessionRepository
	policy Policy
	now    func() time.Time
}

func NewSessionService(repo SessionRepository, policy Policy, now func() time.Time) *SessionService {
	return &SessionService{
		repo:   repo,
		policy: policy,
		now:    now,
	}
}

func (s *SessionService) Create(userID uuid.UUID, userAgent string) (Token, Session, error) {
	token, err := NewToken()
	if err != nil {
		return "", Session{}, err
	}
	session, err := NewSession(token.SessionID(), userID, userAgent, s.now(), s.policy)
	if err != nil {
		return "", Session{}, err
	}
	err = s.repo.Create(session)
	if err != nil {
		return "", Session{}, err
	}
	return token, session, nil
}

func (s *SessionService) Validate(rawToken string) (Session, error) {
	token, err := ParseToken(rawToken)
	if err != nil {
		return Session{}, ErrSessionNotFound
	}
	session, err := s.repo.Find(token.SessionID())
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	if session.IsExpired(now) {
		_, err = s.repo.Delete(session.ID)
		if err != nil && !errors.Is(err, ErrSessionNotFound) {
			return Session{}, err
		}
		return Session{}, ErrSessionNotFound
	}
	if session.Touch(now, s.policy) {
		err = s.repo.Update(session)
		if err != nil {
			return Session{}, err
		}
	}
	return session, nil
}

func (s *SessionService) Revoke(rawToken string) (Session, error) {
	token, err := ParseToken(rawToken)
	if err != nil {
		return Session{}, ErrSessionNotFound
	}
	return s.repo.Delete(token.SessionID())
}

func (s *SessionService) RevokeUserSessions(userID uuid.UUID, exceptSessionID string, createdBefore time.Time) (int, error) {
	if userID == uuid.Nil() {
		return 0, ErrInvalidUserID
	}
	sessions, err := s.repo.FindByUser(userID)
	if err != nil {
		return 0, err
	}
	var revoked int
	for _, session := range sessions {
		if session.ID == exceptSessionID || (!createdBefore.IsZero() && session.CreatedAt.UnixMilli() > createdBefore.UnixMilli()) {
			continue
		}
		_, err = s.repo.Delete(session.ID)
		if errors.Is(err, ErrSessionNotFound) {
			continue
		}
		if err != nil {
			return revoked, err
		}
		revoked++
	}
	return revoked, nil
}
