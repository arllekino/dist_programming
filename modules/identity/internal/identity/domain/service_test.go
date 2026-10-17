package domain

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	identityevents "identity/api/events"
)

type memoryRepository struct {
	users map[uuid.UUID]User
}

func (repo *memoryRepository) Find(id uuid.UUID) (User, error) {
	user, ok := repo.users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (repo *memoryRepository) FindForUpdate(id uuid.UUID) (User, error) {
	return repo.Find(id)
}

func (repo *memoryRepository) Create(user User) error {
	if _, ok := repo.users[user.ID]; ok {
		return ErrUserAlreadyExists
	}
	repo.users[user.ID] = user
	return nil
}

func (repo *memoryRepository) Update(user User) error {
	repo.users[user.ID] = user
	return nil
}

type recordingDispatcher struct {
	events []proto.Message
	err    error
}

func (d *recordingDispatcher) Dispatch(message proto.Message) error {
	if d.err != nil {
		return d.err
	}
	d.events = append(d.events, message)
	return nil
}

func newTestService() (*UserService, *memoryRepository, *recordingDispatcher) {
	repo := &memoryRepository{users: map[uuid.UUID]User{}}
	dispatcher := &recordingDispatcher{}
	svc := NewUserService(repo, dispatcher, func() time.Time { return testNow })
	return svc, repo, dispatcher
}

func TestCreateUser(t *testing.T) {
	svc, repo, dispatcher := newTestService()
	id := uuid.New()

	user, err := svc.CreateUser(id, testProfile(t))
	require.NoError(t, err)
	require.Equal(t, user, repo.users[id])
	require.Len(t, dispatcher.events, 1)
	require.True(t, proto.Equal(&identityevents.UserCreated{UserID: id.String()}, dispatcher.events[0]))

	again, err := svc.CreateUser(id, testProfile(t))
	require.NoError(t, err, "a retry with the same profile is idempotent")
	require.Equal(t, user, again)
	require.Len(t, dispatcher.events, 1)

	other, err := NewProfile("Петр", "Иванов", "petr@example.com")
	require.NoError(t, err)
	_, err = svc.CreateUser(id, other)
	require.ErrorIs(t, err, ErrUserAlreadyExists)

	_, err = svc.CreateUser(uuid.Nil(), testProfile(t))
	require.ErrorIs(t, err, ErrInvalidUserID)
}

func TestCreateUserFailsWhenEventCannotBeDispatched(t *testing.T) {
	svc, _, dispatcher := newTestService()
	dispatcher.err = errors.New("outbox unavailable")

	_, err := svc.CreateUser(uuid.New(), testProfile(t))
	require.ErrorIs(t, err, dispatcher.err)
}

func TestUpdateProfile(t *testing.T) {
	svc, repo, dispatcher := newTestService()
	id := uuid.New()
	_, err := svc.CreateUser(id, testProfile(t))
	require.NoError(t, err)

	unchanged, err := svc.UpdateProfile(id, testProfile(t))
	require.NoError(t, err)
	require.Equal(t, testProfile(t), unchanged.Profile)
	require.Len(t, dispatcher.events, 1, "no event for a no-op update")

	changed, err := NewProfile("Петр", "Иванов", "petr@example.com")
	require.NoError(t, err)
	updated, err := svc.UpdateProfile(id, changed)
	require.NoError(t, err)
	require.Equal(t, changed, updated.Profile)
	require.Equal(t, changed, repo.users[id].Profile)
	require.Len(t, dispatcher.events, 2)
	require.True(t, proto.Equal(&identityevents.UserProfileUpdated{UserID: id.String()}, dispatcher.events[1]))

	_, err = svc.UpdateProfile(uuid.New(), changed)
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestGetUser(t *testing.T) {
	svc, _, _ := newTestService()
	id := uuid.New()
	created, err := svc.CreateUser(id, testProfile(t))
	require.NoError(t, err)

	user, err := svc.GetUser(id)
	require.NoError(t, err)
	require.Equal(t, created, user)

	_, err = svc.GetUser(uuid.New())
	require.ErrorIs(t, err, ErrUserNotFound)
}
