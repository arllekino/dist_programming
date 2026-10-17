package integration_tests

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"

	"session/internal/session/domain"
	sessionredis "session/internal/session/infra/redis"
)

func TestRedisRepository(t *testing.T) {
	repo := sessionredis.NewSessionRepository(t.Context(), testEnv.redis)
	policy, err := domain.NewPolicy(time.Minute, time.Hour)
	require.NoError(t, err)
	userID := uuid.New()

	token, err := domain.NewToken()
	require.NoError(t, err)
	session, err := domain.NewSession(token.SessionID(), userID, "agent", time.Now(), policy)
	require.NoError(t, err)
	require.NoError(t, repo.Create(session))

	found, err := repo.Find(session.ID)
	require.NoError(t, err)
	require.Equal(t, session.UserID, found.UserID)
	require.True(t, session.ExpiresAt.Equal(found.ExpiresAt))
	require.True(t, session.CreatedAt.Equal(found.CreatedAt))

	indexTTL, err := testEnv.redis.TTL(t.Context(), "user_sessions:"+userID.String()).Result()
	require.NoError(t, err)
	require.Greater(t, indexTTL, 59*time.Minute)

	deleted, err := repo.Delete(session.ID)
	require.NoError(t, err)
	require.Equal(t, session.ID, deleted.ID)

	found.ExpiresAt = found.ExpiresAt.Add(time.Minute)
	require.ErrorIs(t, repo.Update(found), domain.ErrSessionNotFound)
	_, err = repo.Find(session.ID)
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
	_, err = repo.Delete(session.ID)
	require.ErrorIs(t, err, domain.ErrSessionNotFound)
}

func TestRedisRepositoryCleansStaleIndexEntries(t *testing.T) {
	repo := sessionredis.NewSessionRepository(t.Context(), testEnv.redis)
	policy, err := domain.NewPolicy(time.Minute, time.Hour)
	require.NoError(t, err)
	userID := uuid.New()
	userKey := "user_sessions:" + userID.String()

	ids := make([]string, 0, 2)
	for range 2 {
		token, err := domain.NewToken()
		require.NoError(t, err)
		session, err := domain.NewSession(token.SessionID(), userID, "", time.Now(), policy)
		require.NoError(t, err)
		require.NoError(t, repo.Create(session))
		ids = append(ids, session.ID)
	}
	require.NoError(t, testEnv.redis.Del(t.Context(), "session:"+ids[0]).Err())

	sessions, err := repo.FindByUser(userID)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, ids[1], sessions[0].ID)

	members, err := testEnv.redis.SMembers(t.Context(), userKey).Result()
	require.NoError(t, err)
	require.Equal(t, []string{ids[1]}, members)
}
