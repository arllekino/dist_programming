package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/logging"
	"github.com/distributed-programming-2026/go-sdk/pkg/uow"

	domainevent "github.com/distributed-programming-2026/lib/event"

	"auth/internal/auth/domain"
	"auth/internal/auth/infra/mysql"
)

const maxAuditLoginLength = 64

type DispatcherFactory interface {
	NewDispatcher(ctx context.Context) domainevent.Dispatcher
}

type Service struct {
	unit              uow.UnitOfWorkWithRepositoryProvider[*mysql.RepositoryProvider]
	dispatcherFactory DispatcherFactory
	hasher            domain.PasswordHasher
	sessions          SessionClient
	identity          IdentityClient
	logger            *slog.Logger
	now               func() time.Time
	dummyHash         domain.PasswordHash
}

func NewService(
	unit uow.UnitOfWorkWithRepositoryProvider[*mysql.RepositoryProvider],
	dispatcherFactory DispatcherFactory,
	hasher domain.PasswordHasher,
	sessions SessionClient,
	identity IdentityClient,
	logger *slog.Logger,
	now func() time.Time,
) (*Service, error) {
	dummyHash, err := hasher.Hash(domain.Password(uuid.New().String()))
	if err != nil {
		return nil, err
	}
	return &Service{
		unit:              unit,
		dispatcherFactory: dispatcherFactory,
		hasher:            hasher,
		sessions:          sessions,
		identity:          identity,
		logger:            logger,
		now:               now,
		dummyHash:         dummyHash,
	}, nil
}

type RegisterResult struct {
	UserID  uuid.UUID
	Session *Session
}

func (s *Service) Register(ctx context.Context, rawLogin, rawPassword string, profile Profile) (RegisterResult, error) {
	login, err := domain.NewLogin(rawLogin)
	if err != nil {
		return RegisterResult{}, err
	}
	password, err := domain.NewPassword(rawPassword)
	if err != nil {
		return RegisterResult{}, err
	}
	err = s.execute(ctx, func(svc *domain.CredentialsService) error {
		return svc.EnsureLoginAvailable(login)
	})
	if err != nil {
		return RegisterResult{}, err
	}

	userID := uuid.New()
	err = s.identity.CreateUser(ctx, userID, profile)
	if err != nil {
		return RegisterResult{}, err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return RegisterResult{}, err
	}
	err = s.execute(ctx, func(svc *domain.CredentialsService) error {
		_, err := svc.Register(userID, login, hash)
		return err
	})
	if err != nil {
		return RegisterResult{}, err
	}

	result := RegisterResult{UserID: userID}
	session, err := s.sessions.Create(ctx, userID)
	if err != nil {
		s.logger.WarnContext(ctx, "session after registration failed", logging.Error(err))
		return result, nil
	}
	result.Session = &session
	return result, nil
}

func (s *Service) Login(ctx context.Context, rawLogin, password string) (Session, error) {
	credentials, reason, err := s.verify(ctx, rawLogin, password)
	if err != nil {
		return Session{}, err
	}
	if reason != "" {
		err = s.execute(ctx, func(svc *domain.CredentialsService) error {
			return svc.RecordLoginFailed(auditLogin(rawLogin), reason)
		})
		return Session{}, errors.Join(domain.ErrInvalidCredentials, err)
	}

	session, err := s.sessions.Create(ctx, credentials.UserID)
	if err != nil {
		return Session{}, err
	}
	err = s.execute(ctx, func(svc *domain.CredentialsService) error {
		return svc.RecordLoginSucceeded(credentials.UserID, session.ID)
	})
	if err != nil {
		_, revokeErr := s.sessions.Revoke(ctx, session.Token)
		return Session{}, errors.Join(err, revokeErr)
	}
	return session, nil
}

func auditLogin(rawLogin string) string {
	login := []rune(strings.ToLower(strings.TrimSpace(rawLogin)))
	return string(login[:min(len(login), maxAuditLoginLength)])
}

func (s *Service) verify(ctx context.Context, rawLogin, password string) (domain.Credentials, string, error) {
	login, err := domain.NewLogin(rawLogin)
	if err != nil {
		s.hasher.Matches(s.dummyHash, password)
		return domain.Credentials{}, domain.LoginFailedUnknownLogin, nil
	}
	var credentials domain.Credentials
	err = s.execute(ctx, func(svc *domain.CredentialsService) error {
		credentials, err = svc.FindByLogin(login)
		return err
	})
	if errors.Is(err, domain.ErrCredentialsNotFound) {
		s.hasher.Matches(s.dummyHash, password)
		return domain.Credentials{}, domain.LoginFailedUnknownLogin, nil
	}
	if err != nil {
		return domain.Credentials{}, "", err
	}
	if !credentials.VerifyPassword(s.hasher, password) {
		return domain.Credentials{}, domain.LoginFailedWrongPassword, nil
	}
	return credentials, "", nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return ErrUnauthenticated
	}
	session, err := s.sessions.Revoke(ctx, token)
	if err != nil {
		return err
	}
	return s.execute(ctx, func(svc *domain.CredentialsService) error {
		return svc.RecordLogout(session.UserID, session.ID)
	})
}

func (s *Service) ChangePassword(ctx context.Context, token, oldPassword, rawNewPassword string) error {
	if token == "" {
		return ErrUnauthenticated
	}
	session, err := s.sessions.Validate(ctx, token)
	if err != nil {
		return err
	}
	newPassword, err := domain.NewPassword(rawNewPassword)
	if err != nil {
		return err
	}

	var credentials domain.Credentials
	err = s.execute(ctx, func(svc *domain.CredentialsService) error {
		credentials, err = svc.FindByUserID(session.UserID)
		return err
	})
	if err != nil {
		return err
	}
	if !credentials.VerifyPassword(s.hasher, oldPassword) {
		return domain.ErrWrongPassword
	}
	if credentials.VerifyPassword(s.hasher, string(newPassword)) {
		return domain.ErrSamePassword
	}
	newHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return err
	}

	err = s.execute(ctx, func(svc *domain.CredentialsService) error {
		credentials, err = svc.ChangePassword(session.UserID, credentials.PasswordHash, newHash, session.ID)
		return err
	})
	if err != nil {
		return err
	}

	err = s.sessions.RevokeUserSessions(ctx, session.UserID, session.ID, credentials.PasswordChangedAt)
	if err != nil {
		s.logger.WarnContext(ctx, "revoking sessions after password change failed; relying on PasswordChanged",
			slog.String("user_id", session.UserID.String()), logging.Error(err))
	}
	return nil
}

func (s *Service) execute(ctx context.Context, operation func(*domain.CredentialsService) error) error {
	return s.unit.ExecuteWithRepositoryProvider(ctx, func(provider *mysql.RepositoryProvider) error {
		return operation(domain.NewCredentialsService(
			provider.Credentials(ctx),
			s.dispatcherFactory.NewDispatcher(ctx),
			s.now,
		))
	})
}
