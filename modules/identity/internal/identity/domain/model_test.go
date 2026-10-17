package domain

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

const (
	ivan   = "Ivan"
	petrov = "Petrov"
	email  = "a@b.c"
)

func TestNewProfile(t *testing.T) {
	profile, err := NewProfile("  Иван ", " Петров", " Ivan.Petrov@Example.COM ")
	require.NoError(t, err)
	require.Equal(t, Profile{FirstName: "Иван", LastName: "Петров", Email: "ivan.petrov@example.com"}, profile)

	tests := []struct {
		name      string
		firstName string
		lastName  string
		email     string
		err       error
	}{
		{name: "empty first name", firstName: " ", lastName: petrov, email: email, err: ErrInvalidFirstName},
		{name: "long first name", firstName: strings.Repeat("я", 101), lastName: petrov, email: email, err: ErrInvalidFirstName},
		{name: "empty last name", firstName: ivan, lastName: "", email: email, err: ErrInvalidLastName},
		{name: "missing at", firstName: ivan, lastName: petrov, email: "ivan.example.com", err: ErrInvalidEmail},
		{name: "display name", firstName: ivan, lastName: petrov, email: "Ivan <ivan@example.com>", err: ErrInvalidEmail},
		{name: "long email", firstName: ivan, lastName: petrov, email: strings.Repeat("a", 250) + "@b.cd", err: ErrInvalidEmail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProfile(tt.firstName, tt.lastName, tt.email)
			require.ErrorIs(t, err, tt.err)
		})
	}

	_, err = NewProfile(strings.Repeat("я", 100), "P", email)
	require.NoError(t, err, "length is counted in characters, not bytes")
}

func TestNewUser(t *testing.T) {
	id := uuid.New()
	profile := testProfile(t)

	user, err := NewUser(id, profile, testNow)
	require.NoError(t, err)
	require.Equal(t, id, user.ID)
	require.Equal(t, profile, user.Profile)
	require.Equal(t, StatusActive, user.Status)
	require.Equal(t, testNow, user.CreatedAt)
	require.Equal(t, testNow, user.UpdatedAt)

	_, err = NewUser(uuid.Nil(), profile, testNow)
	require.ErrorIs(t, err, ErrInvalidUserID)
}

func TestUserUpdateProfile(t *testing.T) {
	user, err := NewUser(uuid.New(), testProfile(t), testNow)
	require.NoError(t, err)

	later := testNow.Add(time.Hour)
	require.False(t, user.UpdateProfile(testProfile(t), later))
	require.Equal(t, testNow, user.UpdatedAt)

	changed, err := NewProfile("Петр", "Иванов", "petr@example.com")
	require.NoError(t, err)
	require.True(t, user.UpdateProfile(changed, later))
	require.Equal(t, changed, user.Profile)
	require.Equal(t, later, user.UpdatedAt)
	require.Equal(t, testNow, user.CreatedAt)
}

func testProfile(t *testing.T) Profile {
	t.Helper()
	profile, err := NewProfile("Иван", "Петров", "ivan@example.com")
	require.NoError(t, err)
	return profile
}
