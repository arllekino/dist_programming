package domain

import (
	"errors"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/lib/event"

	identityevents "identity/api/events"
)

type UserService struct {
	repo       UserRepository
	dispatcher event.Dispatcher
	now        func() time.Time
}

func NewUserService(repo UserRepository, dispatcher event.Dispatcher, now func() time.Time) *UserService {
	return &UserService{
		repo:       repo,
		dispatcher: dispatcher,
		now:        now,
	}
}

func (s *UserService) CreateUser(id uuid.UUID, profile Profile) (User, error) {
	existing, err := s.repo.Find(id)
	if err == nil {
		if existing.Profile != profile {
			return User{}, ErrUserAlreadyExists
		}
		return existing, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return User{}, err
	}

	user, err := NewUser(id, profile, s.now())
	if err != nil {
		return User{}, err
	}
	err = s.repo.Create(user)
	if err != nil {
		return User{}, err
	}
	err = s.dispatcher.Dispatch(&identityevents.UserCreated{
		UserID: user.ID.String(),
	})
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *UserService) UpdateProfile(id uuid.UUID, profile Profile) (User, error) {
	user, err := s.repo.FindForUpdate(id)
	if err != nil {
		return User{}, err
	}
	if !user.UpdateProfile(profile, s.now()) {
		return user, nil
	}
	err = s.repo.Update(user)
	if err != nil {
		return User{}, err
	}
	err = s.dispatcher.Dispatch(&identityevents.UserProfileUpdated{
		UserID: user.ID.String(),
	})
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *UserService) GetUser(id uuid.UUID) (User, error) {
	return s.repo.Find(id)
}
