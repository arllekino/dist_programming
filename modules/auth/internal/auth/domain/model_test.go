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

const ivan = "ivan"

func TestNewLogin(t *testing.T) {
	for _, tt := range []struct {
		raw      string
		expected Login
	}{
		{raw: ivan, expected: ivan},
		{raw: " Ivan.Petrov\t", expected: "ivan.petrov"},
		{raw: "ivan@example.com", expected: "ivan@example.com"},
		{raw: "a-b_c", expected: "a-b_c"},
		{raw: strings.Repeat("a", 64), expected: Login(strings.Repeat("a", 64))},
		{raw: "0day", expected: "0day"},
	} {
		login, err := NewLogin(tt.raw)
		require.NoError(t, err, tt.raw)
		require.Equal(t, tt.expected, login)
	}

	for _, raw := range []string{"", "ab", strings.Repeat("a", 65), ".ivan", "иван", "ivan petrov", "ivan!"} {
		_, err := NewLogin(raw)
		require.ErrorIs(t, err, ErrInvalidLogin, raw)
	}
}

func TestNewPassword(t *testing.T) {
	for _, raw := range []string{"password1", "пароль123", "12345678a", strings.Repeat("a", 71) + "1"} {
		_, err := NewPassword(raw)
		require.NoError(t, err, raw)
	}

	for _, raw := range []string{
		"",
		"short1",
		"onlyletters",
		"1234567890",
		strings.Repeat("a", 72) + "1",
		strings.Repeat("я", 36) + "1",
	} {
		_, err := NewPassword(raw)
		require.ErrorIs(t, err, ErrInvalidPassword, raw)
	}
}

func TestNewCredentials(t *testing.T) {
	userID := uuid.New()

	credentials, err := NewCredentials(userID, ivan, "hash", testNow)
	require.NoError(t, err)
	require.Equal(t, userID, credentials.UserID)
	require.Equal(t, Login(ivan), credentials.Login)
	require.Equal(t, PasswordHash("hash"), credentials.PasswordHash)
	require.Equal(t, testNow, credentials.CreatedAt)
	require.Equal(t, testNow, credentials.PasswordChangedAt)

	_, err = NewCredentials(uuid.Nil(), ivan, "hash", testNow)
	require.ErrorIs(t, err, ErrInvalidUserID)
}

func TestCredentialsPassword(t *testing.T) {
	hasher := plainHasher{}
	credentials, err := NewCredentials(uuid.New(), ivan, "hash:password1", testNow)
	require.NoError(t, err)

	require.True(t, credentials.VerifyPassword(hasher, "password1"))
	require.False(t, credentials.VerifyPassword(hasher, "password2"))

	later := testNow.Add(time.Hour)
	credentials.ChangePassword("hash:password2", later)
	require.True(t, credentials.VerifyPassword(hasher, "password2"))
	require.False(t, credentials.VerifyPassword(hasher, "password1"))
	require.Equal(t, later, credentials.PasswordChangedAt)
	require.Equal(t, testNow, credentials.CreatedAt)
}

type plainHasher struct{}

func (plainHasher) Hash(password Password) (PasswordHash, error) {
	return PasswordHash("hash:" + password), nil
}

func (plainHasher) Matches(hash PasswordHash, candidate string) bool {
	return string(hash) == "hash:"+candidate
}
