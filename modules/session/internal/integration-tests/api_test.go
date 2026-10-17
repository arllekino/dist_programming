package integration_tests

import (
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	internalapi "session/api/server/internalapi"
)

func TestSessionLifecycle(t *testing.T) {
	client := sessionClient(t)
	userID := uuid.New().String()

	created := createSession(t, client, userID)
	require.NotContains(t, created.SessionID, created.Token)

	validated := requireValid(t, client, created.Token, userID)
	require.Equal(t, created.SessionID, validated.SessionID)

	ctx := callContext(t)
	exists, err := testEnv.redis.Exists(ctx, "session:"+created.SessionID, "session:"+created.Token).Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, exists)
	ttl, err := testEnv.redis.PTTL(ctx, "session:"+created.SessionID).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl, idleTTL)
	members, err := testEnv.redis.SMembers(ctx, "user_sessions:"+userID).Result()
	require.NoError(t, err)
	require.Equal(t, []string{created.SessionID}, members)

	revoked, err := client.RevokeSession(callContext(t), &internalapi.RevokeSessionRequest{Token: created.Token})
	require.NoError(t, err)
	require.Equal(t, userID, revoked.UserID)
	require.Equal(t, created.SessionID, revoked.SessionID)

	requireInvalid(t, client, created.Token)
	_, err = client.RevokeSession(callContext(t), &internalapi.RevokeSessionRequest{Token: created.Token})
	requireCode(t, codes.NotFound, err)
	members, err = testEnv.redis.SMembers(callContext(t), "user_sessions:"+userID).Result()
	require.NoError(t, err)
	require.Empty(t, members)
}

func TestInvalidRequests(t *testing.T) {
	client := sessionClient(t)

	for _, userID := range []string{"", "not-a-uuid", uuid.Nil().String()} {
		_, err := client.CreateSession(callContext(t), &internalapi.CreateSessionRequest{UserID: userID})
		requireCode(t, codes.InvalidArgument, err)
	}
	for _, token := range []string{"", "malformed", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		requireInvalid(t, client, token)
	}
	_, err := client.RevokeUserSessions(callContext(t), &internalapi.RevokeUserSessionsRequest{UserID: "bad"})
	requireCode(t, codes.InvalidArgument, err)
}

func TestSlidingAndAbsoluteExpiration(t *testing.T) {
	t.Parallel()
	client := sessionClient(t)
	userID := uuid.New().String()

	active := createSession(t, client, userID)
	idle := createSession(t, client, userID)
	createdAt := time.Now()

	time.Sleep(idleTTL/2 + 500*time.Millisecond)
	slid := requireValid(t, client, active.Token, userID)
	require.Greater(t, slid.ExpiresAt, active.ExpiresAt)

	time.Sleep(idleTTL/2 + 500*time.Millisecond)
	requireInvalid(t, client, idle.Token)
	requireValid(t, client, active.Token, userID)

	require.Eventually(t, func() bool {
		_, err := client.ValidateSession(callContext(t), &internalapi.ValidateSessionRequest{Token: active.Token})
		return err != nil
	}, absoluteTTL, 500*time.Millisecond)
	require.GreaterOrEqual(t, time.Since(createdAt), absoluteTTL-time.Second)
}

func TestRevokeUserSessions(t *testing.T) {
	client := sessionClient(t)
	userID := uuid.New().String()
	otherUserID := uuid.New().String()

	first := createSession(t, client, userID)
	second := createSession(t, client, userID)
	kept := createSession(t, client, userID)
	foreign := createSession(t, client, otherUserID)

	response, err := client.RevokeUserSessions(callContext(t), &internalapi.RevokeUserSessionsRequest{
		UserID:          userID,
		ExceptSessionID: kept.SessionID,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, response.Revoked)

	requireInvalid(t, client, first.Token)
	requireInvalid(t, client, second.Token)
	requireValid(t, client, kept.Token, userID)
	requireValid(t, client, foreign.Token, otherUserID)

	response, err = client.RevokeUserSessions(callContext(t), &internalapi.RevokeUserSessionsRequest{
		UserID:          userID,
		ExceptSessionID: kept.SessionID,
	})
	require.NoError(t, err)
	require.Zero(t, response.Revoked)
}
