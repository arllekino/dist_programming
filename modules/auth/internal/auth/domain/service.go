package domain

import (
	"errors"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/lib/event"

	authevents "auth/api/events"
)

const (
	LoginFailedUnknownLogin  = "unknown_login"
	LoginFailedWrongPassword = "wrong_password"
)

type CredentialsService struct {
	repo       CredentialsRepository
	dispatcher event.Dispatcher
	now        func() time.Time
}

func NewCredentialsService(repo CredentialsRepository, dispatcher event.Dispatcher, now func() time.Time) *CredentialsService {
	return &CredentialsService{
		repo:       repo,
		dispatcher: dispatcher,
		now:        now,
	}
}

func (s *CredentialsService) EnsureLoginAvailable(login Login) error {
	_, err := s.repo.FindByLogin(login)
	if err == nil {
		return ErrLoginTaken
	}
	if errors.Is(err, ErrCredentialsNotFound) {
		return nil
	}
	return err
}

func (s *CredentialsService) Register(
	userID uuid.UUID,
	login Login,
	hash PasswordHash,
) (Credentials, error) {
	credentials, err := NewCredentials(userID, login, hash, s.now())
	if err != nil {
		return Credentials{}, err
	}
	err = s.repo.Create(credentials)
	if err != nil {
		return Credentials{}, err
	}
	err = s.dispatcher.Dispatch(&authevents.UserRegistered{
		UserID: userID.String(),
	})
	if err != nil {
		return Credentials{}, err
	}
	return credentials, nil
}

func (s *CredentialsService) FindByLogin(login Login) (Credentials, error) {
	return s.repo.FindByLogin(login)
}

func (s *CredentialsService) FindByUserID(userID uuid.UUID) (Credentials, error) {
	return s.repo.FindByUserID(userID)
}

func (s *CredentialsService) RecordLoginSucceeded(userID uuid.UUID, sessionID string) error {
	return s.dispatcher.Dispatch(&authevents.UserLoggedIn{
		UserID:    userID.String(),
		SessionID: sessionID,
	})
}

func (s *CredentialsService) RecordLoginFailed(login, reason string) error {
	return s.dispatcher.Dispatch(&authevents.UserLoginFailed{
		Login:  login,
		Reason: reason,
	})
}

func (s *CredentialsService) RecordLogout(userID uuid.UUID, sessionID string) error {
	return s.dispatcher.Dispatch(&authevents.UserLoggedOut{
		UserID:    userID.String(),
		SessionID: sessionID,
	})
}

func (s *CredentialsService) ChangePassword(userID uuid.UUID, verifiedHash, newHash PasswordHash, keepSessionID string) (Credentials, error) {
	credentials, err := s.repo.FindByUserIDForUpdate(userID)
	if err != nil {
		return Credentials{}, err
	}
	if credentials.PasswordHash != verifiedHash {
		return Credentials{}, ErrWrongPassword
	}
	credentials.ChangePassword(newHash, s.now())
	err = s.repo.Update(credentials)
	if err != nil {
		return Credentials{}, err
	}
	err = s.dispatcher.Dispatch(&authevents.PasswordChanged{
		UserID:        userID.String(),
		ChangedAt:     credentials.PasswordChangedAt.UnixMilli(),
		KeepSessionID: keepSessionID,
	})
	if err != nil {
		return Credentials{}, err
	}
	return credentials, nil
}
