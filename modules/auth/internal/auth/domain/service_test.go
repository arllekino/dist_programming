package domain

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	authevents "auth/api/events"
)

type memoryRepository struct {
	byUserID map[uuid.UUID]Credentials
}

func (repo *memoryRepository) FindByLogin(login Login) (Credentials, error) {
	for _, c := range repo.byUserID {
		if c.Login == login {
			return c, nil
		}
	}
	return Credentials{}, ErrCredentialsNotFound
}

func (repo *memoryRepository) FindByUserID(userID uuid.UUID) (Credentials, error) {
	c, ok := repo.byUserID[userID]
	if !ok {
		return Credentials{}, ErrCredentialsNotFound
	}
	return c, nil
}

func (repo *memoryRepository) FindByUserIDForUpdate(userID uuid.UUID) (Credentials, error) {
	return repo.FindByUserID(userID)
}

func (repo *memoryRepository) Create(c Credentials) error {
	if _, err := repo.FindByLogin(c.Login); err == nil {
		return ErrLoginTaken
	}
	repo.byUserID[c.UserID] = c
	return nil
}

func (repo *memoryRepository) Update(c Credentials) error {
	repo.byUserID[c.UserID] = c
	return nil
}

type recordingDispatcher struct {
	events []proto.Message
}

func (d *recordingDispatcher) Dispatch(message proto.Message) error {
	d.events = append(d.events, message)
	return nil
}

func newTestService() (*CredentialsService, *memoryRepository, *recordingDispatcher) {
	repo := &memoryRepository{byUserID: map[uuid.UUID]Credentials{}}
	dispatcher := &recordingDispatcher{}
	return NewCredentialsService(repo, dispatcher, func() time.Time { return testNow }), repo, dispatcher
}

func TestRegister(t *testing.T) {
	svc, repo, dispatcher := newTestService()
	userID := uuid.New()

	require.NoError(t, svc.EnsureLoginAvailable(ivan))
	credentials, err := svc.Register(userID, ivan, "hash:password1")
	require.NoError(t, err)
	require.Equal(t, credentials, repo.byUserID[userID])
	require.Len(t, dispatcher.events, 1)
	require.True(t, proto.Equal(&authevents.UserRegistered{UserID: userID.String()}, dispatcher.events[0]))

	require.ErrorIs(t, svc.EnsureLoginAvailable(ivan), ErrLoginTaken)
	_, err = svc.Register(uuid.New(), ivan, "hash:password1")
	require.ErrorIs(t, err, ErrLoginTaken)
	require.Len(t, dispatcher.events, 1)

	_, err = svc.Register(uuid.Nil(), "petr", "hash:password1")
	require.ErrorIs(t, err, ErrInvalidUserID)
}

func TestRecordEvents(t *testing.T) {
	svc, _, dispatcher := newTestService()
	userID := uuid.New()

	require.NoError(t, svc.RecordLoginSucceeded(userID, "session-1"))
	require.NoError(t, svc.RecordLoginFailed(ivan, LoginFailedWrongPassword))
	require.NoError(t, svc.RecordLogout(userID, "session-1"))

	expected := []proto.Message{
		&authevents.UserLoggedIn{UserID: userID.String(), SessionID: "session-1"},
		&authevents.UserLoginFailed{Login: ivan, Reason: LoginFailedWrongPassword},
		&authevents.UserLoggedOut{UserID: userID.String(), SessionID: "session-1"},
	}
	require.Len(t, dispatcher.events, len(expected))
	for i := range expected {
		require.True(t, proto.Equal(expected[i], dispatcher.events[i]), "event %d", i)
	}
}

func TestChangePassword(t *testing.T) {
	svc, repo, dispatcher := newTestService()
	userID := uuid.New()
	registered, err := svc.Register(userID, ivan, "hash:password1")
	require.NoError(t, err)

	later := testNow.Add(time.Hour)
	svc.now = func() time.Time { return later }
	changed, err := svc.ChangePassword(userID, registered.PasswordHash, "hash:password2", "kept-session")
	require.NoError(t, err)
	require.Equal(t, PasswordHash("hash:password2"), changed.PasswordHash)
	require.Equal(t, later, changed.PasswordChangedAt)
	require.Equal(t, changed, repo.byUserID[userID])
	require.True(t, proto.Equal(&authevents.PasswordChanged{
		UserID:        userID.String(),
		ChangedAt:     later.UnixMilli(),
		KeepSessionID: "kept-session",
	}, dispatcher.events[len(dispatcher.events)-1]))
}

func TestChangePasswordDetectsConcurrentChange(t *testing.T) {
	svc, repo, dispatcher := newTestService()
	userID := uuid.New()
	registered, err := svc.Register(userID, ivan, "hash:password1")
	require.NoError(t, err)

	_, err = svc.ChangePassword(userID, registered.PasswordHash, "hash:password2", "")
	require.NoError(t, err)
	events := len(dispatcher.events)

	_, err = svc.ChangePassword(userID, registered.PasswordHash, "hash:password3", "")
	require.ErrorIs(t, err, ErrWrongPassword)
	require.Equal(t, PasswordHash("hash:password2"), repo.byUserID[userID].PasswordHash)
	require.Len(t, dispatcher.events, events)

	_, err = svc.ChangePassword(uuid.New(), "", "hash:password3", "")
	require.ErrorIs(t, err, ErrCredentialsNotFound)
}

func TestFindCredentials(t *testing.T) {
	svc, _, _ := newTestService()
	userID := uuid.New()
	registered, err := svc.Register(userID, ivan, "hash:password1")
	require.NoError(t, err)

	byLogin, err := svc.FindByLogin(ivan)
	require.NoError(t, err)
	require.Equal(t, registered, byLogin)
	byUserID, err := svc.FindByUserID(userID)
	require.NoError(t, err)
	require.Equal(t, registered, byUserID)

	_, err = svc.FindByLogin("petr")
	require.True(t, errors.Is(err, ErrCredentialsNotFound))
}
