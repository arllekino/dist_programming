package domain

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
)

type memoryRepository struct {
	sessions map[string]Session
	updates  int
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{sessions: map[string]Session{}}
}

func (repo *memoryRepository) Create(s Session) error {
	repo.sessions[s.ID] = s
	return nil
}

func (repo *memoryRepository) Find(id string) (Session, error) {
	s, ok := repo.sessions[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return s, nil
}

func (repo *memoryRepository) Update(s Session) error {
	if _, ok := repo.sessions[s.ID]; !ok {
		return ErrSessionNotFound
	}
	repo.updates++
	repo.sessions[s.ID] = s
	return nil
}

func (repo *memoryRepository) Delete(id string) (Session, error) {
	s, ok := repo.sessions[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	delete(repo.sessions, id)
	return s, nil
}

func (repo *memoryRepository) FindByUser(userID uuid.UUID) ([]Session, error) {
	var result []Session
	for _, s := range repo.sessions {
		if s.UserID == userID {
			result = append(result, s)
		}
	}
	return result, nil
}

func newTestService(t *testing.T) (*SessionService, *memoryRepository, *testClock) {
	t.Helper()
	repo := newMemoryRepository()
	clock := &testClock{now: testNow}
	return NewSessionService(repo, testPolicy(t), clock.Now), repo, clock
}

func TestCreateAndValidate(t *testing.T) {
	svc, repo, _ := newTestService(t)
	userID := uuid.New()

	token, created, err := svc.Create(userID, "curl")
	require.NoError(t, err)
	require.Equal(t, token.SessionID(), created.ID)
	require.Contains(t, repo.sessions, created.ID)

	validated, err := svc.Validate(string(token))
	require.NoError(t, err)
	require.Equal(t, created, validated)
	require.Zero(t, repo.updates)

	_, _, err = svc.Create(uuid.Nil(), "")
	require.ErrorIs(t, err, ErrInvalidUserID)
}

func TestValidateRejectsUnknownAndMalformedTokens(t *testing.T) {
	svc, _, _ := newTestService(t)
	unknown, err := NewToken()
	require.NoError(t, err)

	for _, token := range []string{"", "malformed", string(unknown)} {
		_, err = svc.Validate(token)
		require.ErrorIs(t, err, ErrSessionNotFound)
	}
}

func TestValidateSlidesExpiration(t *testing.T) {
	svc, repo, clock := newTestService(t)
	token, _, err := svc.Create(uuid.New(), "")
	require.NoError(t, err)

	clock.now = testNow.Add(40 * time.Minute)
	session, err := svc.Validate(string(token))
	require.NoError(t, err)
	require.Equal(t, clock.now.Add(time.Hour), session.ExpiresAt)
	require.Equal(t, 1, repo.updates)
	require.Equal(t, session, repo.sessions[session.ID])

	clock.now = clock.now.Add(59 * time.Minute)
	_, err = svc.Validate(string(token))
	require.NoError(t, err, "a slid session must outlive the original idle TTL")
}

func TestValidateDeletesExpiredSession(t *testing.T) {
	svc, repo, clock := newTestService(t)
	token, _, err := svc.Create(uuid.New(), "")
	require.NoError(t, err)

	clock.now = testNow.Add(time.Hour)
	_, err = svc.Validate(string(token))
	require.ErrorIs(t, err, ErrSessionNotFound)
	require.Empty(t, repo.sessions)
}

func TestRevoke(t *testing.T) {
	svc, _, _ := newTestService(t)
	userID := uuid.New()
	token, _, err := svc.Create(userID, "")
	require.NoError(t, err)

	revoked, err := svc.Revoke(string(token))
	require.NoError(t, err)
	require.Equal(t, userID, revoked.UserID)

	_, err = svc.Validate(string(token))
	require.ErrorIs(t, err, ErrSessionNotFound)
	_, err = svc.Revoke(string(token))
	require.ErrorIs(t, err, ErrSessionNotFound)
	_, err = svc.Revoke("malformed")
	require.ErrorIs(t, err, ErrSessionNotFound)
}

func TestRevokeUserSessions(t *testing.T) {
	svc, repo, clock := newTestService(t)
	userID := uuid.New()
	otherUserID := uuid.New()

	_, first, err := svc.Create(userID, "first")
	require.NoError(t, err)
	_, kept, err := svc.Create(userID, "kept")
	require.NoError(t, err)
	_, foreign, err := svc.Create(otherUserID, "foreign")
	require.NoError(t, err)

	changedAt := testNow.Add(time.Minute)
	clock.now = testNow.Add(2 * time.Minute)
	_, later, err := svc.Create(userID, "created after password change")
	require.NoError(t, err)

	revoked, err := svc.RevokeUserSessions(userID, kept.ID, changedAt)
	require.NoError(t, err)
	require.Equal(t, 1, revoked)
	require.NotContains(t, repo.sessions, first.ID)
	require.Contains(t, repo.sessions, kept.ID)
	require.Contains(t, repo.sessions, later.ID)
	require.Contains(t, repo.sessions, foreign.ID)

	revoked, err = svc.RevokeUserSessions(userID, kept.ID, changedAt)
	require.NoError(t, err)
	require.Zero(t, revoked, "redelivery must be a no-op")

	revoked, err = svc.RevokeUserSessions(userID, "", time.Time{})
	require.NoError(t, err)
	require.Equal(t, 2, revoked)

	clock.now = testNow.Add(time.Hour + 500*time.Microsecond)
	_, sameMillisecond, err := svc.Create(userID, "")
	require.NoError(t, err)
	revoked, err = svc.RevokeUserSessions(userID, "", testNow.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, 1, revoked, "a tie at millisecond precision revokes the session")
	require.NotContains(t, repo.sessions, sameMillisecond.ID)

	_, err = svc.RevokeUserSessions(uuid.Nil(), "", time.Time{})
	require.ErrorIs(t, err, ErrInvalidUserID)
}

type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time {
	return c.now
}
