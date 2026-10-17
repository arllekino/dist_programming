package domain

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
)

var testNow = time.Date(
	2026,
	10,
	14,
	12,
	0,
	0,
	0,
	time.UTC,
)

func testPolicy(t *testing.T) Policy {
	t.Helper()
	policy, err := NewPolicy(time.Hour, 4*time.Hour)
	require.NoError(t, err)
	return policy
}

func TestNewPolicy(t *testing.T) {
	_, err := NewPolicy(0, time.Hour)
	require.ErrorIs(t, err, ErrInvalidPolicy)
	_, err = NewPolicy(2*time.Hour, time.Hour)
	require.ErrorIs(t, err, ErrInvalidPolicy)
	_, err = NewPolicy(time.Hour, time.Hour)
	require.NoError(t, err)
}

func TestNewSession(t *testing.T) {
	policy := testPolicy(t)
	userID := uuid.New()

	session, err := NewSession("id", userID, testNow, policy)
	require.NoError(t, err)
	require.Equal(t, userID, session.UserID)
	require.Equal(t, testNow, session.CreatedAt)
	require.Equal(t, testNow.Add(time.Hour), session.ExpiresAt)
	require.Equal(t, testNow.Add(4*time.Hour), session.AbsoluteExpiresAt)

	_, err = NewSession("id", uuid.Nil(), testNow, policy)
	require.ErrorIs(t, err, ErrInvalidUserID)
}

func TestSessionIsExpired(t *testing.T) {
	session, err := NewSession("id", uuid.New(), testNow, testPolicy(t))
	require.NoError(t, err)

	require.False(t, session.IsExpired(testNow.Add(time.Hour-time.Nanosecond)))
	require.True(t, session.IsExpired(testNow.Add(time.Hour)))
}

func TestSessionTouch(t *testing.T) {
	policy := testPolicy(t)

	tests := []struct {
		name      string
		at        time.Duration
		changed   bool
		expiresAt time.Duration
	}{
		{name: "fresh session is not rewritten", at: 10 * time.Minute, expiresAt: time.Hour},
		{name: "exactly half remaining is not rewritten", at: 30 * time.Minute, expiresAt: time.Hour},
		{name: "less than half remaining slides", at: 31 * time.Minute, changed: true, expiresAt: 91 * time.Minute},
		{name: "expired session is not revived", at: time.Hour, expiresAt: time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := NewSession("id", uuid.New(), testNow, policy)
			require.NoError(t, err)

			require.Equal(t, tt.changed, session.Touch(testNow.Add(tt.at), policy))
			require.Equal(t, testNow.Add(tt.expiresAt), session.ExpiresAt)
		})
	}
}

func TestSessionTouchIsCappedByAbsoluteExpiration(t *testing.T) {
	policy := testPolicy(t)
	session, err := NewSession("id", uuid.New(), testNow, policy)
	require.NoError(t, err)

	now := testNow
	for range 10 {
		now = now.Add(45 * time.Minute)
		if session.IsExpired(now) {
			break
		}
		session.Touch(now, policy)
	}
	require.Equal(t, testNow.Add(4*time.Hour), session.ExpiresAt)
	require.True(t, session.IsExpired(testNow.Add(4*time.Hour)))
	require.False(t, session.Touch(testNow.Add(4*time.Hour-time.Minute), policy))
}

func TestToken(t *testing.T) {
	token, err := NewToken()
	require.NoError(t, err)
	other, err := NewToken()
	require.NoError(t, err)
	require.NotEqual(t, token, other)

	parsed, err := ParseToken(string(token))
	require.NoError(t, err)
	require.Equal(t, token, parsed)
	require.Len(t, token.SessionID(), 64)
	require.Equal(t, token.SessionID(), parsed.SessionID())
	require.NotContains(t, token.SessionID(), string(token))

	for _, invalid := range []string{"", "short", string(token) + "x", strings.Repeat("!", 43)} {
		_, err = ParseToken(invalid)
		require.ErrorIs(t, err, ErrInvalidToken, invalid)
	}
}
