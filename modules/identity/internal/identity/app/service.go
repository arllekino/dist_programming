package app

import (
	"context"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/uow"

	domainevent "github.com/distributed-programming-2026/lib/event"

	"identity/internal/identity/domain"
	"identity/internal/identity/infra/mysql"
)

type DispatcherFactory interface {
	NewDispatcher(ctx context.Context) domainevent.Dispatcher
}

type Service struct {
	unit              uow.UnitOfWorkWithRepositoryProvider[*mysql.RepositoryProvider]
	dispatcherFactory DispatcherFactory
	now               func() time.Time
}

func NewService(
	unit uow.UnitOfWorkWithRepositoryProvider[*mysql.RepositoryProvider],
	dispatcherFactory DispatcherFactory,
	now func() time.Time,
) *Service {
	return &Service{
		unit:              unit,
		dispatcherFactory: dispatcherFactory,
		now:               now,
	}
}

func (s *Service) CreateUser(ctx context.Context, id uuid.UUID, profile domain.Profile) (domain.User, error) {
	return s.execute(ctx, func(svc *domain.UserService) (domain.User, error) {
		return svc.CreateUser(id, profile)
	})
}

func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, profile domain.Profile) (domain.User, error) {
	return s.execute(ctx, func(svc *domain.UserService) (domain.User, error) {
		return svc.UpdateProfile(id, profile)
	})
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return s.execute(ctx, func(svc *domain.UserService) (domain.User, error) {
		return svc.GetUser(id)
	})
}

func (s *Service) execute(ctx context.Context, operation func(*domain.UserService) (domain.User, error)) (domain.User, error) {
	var user domain.User
	err := s.unit.ExecuteWithRepositoryProvider(ctx, func(provider *mysql.RepositoryProvider) error {
		svc := domain.NewUserService(
			provider.User(ctx),
			s.dispatcherFactory.NewDispatcher(ctx),
			s.now,
		)

		var err error
		user, err = operation(svc)
		return err
	})
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}
