package integration_tests

import (
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"

	authevents "auth/api/events"
	publicapi "auth/api/server/publicapi"
)

func TestRegisterCreatesProfileCredentialsAndSession(t *testing.T) {
	user := register(t)

	session, ok := validateSession(t, user.token)
	require.True(t, ok)
	require.Equal(t, user.userID, session.UserID)

	profile := getIdentityUser(t, user.userID)
	require.Equal(t, "Иван", profile.Profile.FirstName)
	require.Equal(t, user.login+"@example.com", profile.Profile.Email)

	const query = `
		SELECT
			password_hash
		FROM
			credentials
		WHERE
			user_id = ?
	`
	var hash string
	userID := uuid.MustParse(user.userID)
	err := testEnv.connector.TransactionalClient().GetContext(t.Context(), "test.credentials", &hash, query, userID[:])
	require.NoError(t, err)
	require.NotContains(t, hash, user.password)

	registered := outboxEvents(t, "UserRegistered", func() *authevents.UserRegistered { return &authevents.UserRegistered{} })
	require.Contains(t, userIDs(registered), user.userID)
}

func TestRegisterValidation(t *testing.T) {
	existing := register(t)
	client := authClient(t)

	tests := []struct {
		name   string
		body   publicapi.RegisterJSONRequestBody
		status int
	}{
		{name: "invalid login", body: registerBody("ab", "password1", "a@example.com"), status: http.StatusBadRequest},
		{name: "weak password", body: registerBody("new-user-1", "password", "a@example.com"), status: http.StatusBadRequest},
		{name: "invalid email from identity", body: registerBody("new-user-2", "password1", "broken"), status: http.StatusBadRequest},
		{name: "login taken case-insensitively", body: registerBody(strings.ToUpper(existing.login), "password1", "b@example.com"), status: http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := client.RegisterWithResponse(t.Context(), tt.body)
			require.NoError(t, err)
			require.Equal(t, tt.status, response.StatusCode(), string(response.Body))
			if tt.status == http.StatusBadRequest {
				require.NotEmpty(t, response.JSON400.Message)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	user := register(t)

	response := login(t, " "+strings.ToUpper(user.login)+" ", user.password)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	require.NotEqual(t, user.token, response.JSON200.Token)
	require.True(t, response.JSON200.ExpiresAt.After(time.Now()))

	session, ok := validateSession(t, response.JSON200.Token)
	require.True(t, ok)
	require.Equal(t, user.userID, session.UserID)

	loggedIn := outboxEvents(t, "UserLoggedIn", func() *authevents.UserLoggedIn { return &authevents.UserLoggedIn{} })
	require.Contains(t, sessionIDs(loggedIn), session.SessionID)
}

func TestLoginFailures(t *testing.T) {
	user := register(t)
	unknown := "unknown-" + user.login

	for _, attempt := range []struct{ login, password string }{
		{login: user.login, password: "wrong-password1"},
		{login: unknown, password: user.password},
		{login: "", password: user.password},
	} {
		response := login(t, attempt.login, attempt.password)
		require.Equal(t, http.StatusUnauthorized, response.StatusCode())
		require.Equal(t, "invalid login or password", response.JSON401.Message, "failures must be indistinguishable")
	}

	failed := outboxEvents(t, "UserLoginFailed", func() *authevents.UserLoginFailed { return &authevents.UserLoginFailed{} })
	reasons := map[string]string{}
	for _, e := range failed {
		reasons[e.Login] = e.Reason
	}
	require.Equal(t, "wrong_password", reasons[user.login])
	require.Equal(t, "unknown_login", reasons[unknown])
}

func TestLogout(t *testing.T) {
	user := register(t)
	session, ok := validateSession(t, user.token)
	require.True(t, ok)
	client := authClient(t)

	response, err := client.LogoutWithResponse(t.Context(), bearer(user.token))
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, response.StatusCode(), string(response.Body))

	_, ok = validateSession(t, user.token)
	require.False(t, ok)

	response, err = client.LogoutWithResponse(t.Context(), bearer(user.token))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode())

	response, err = client.LogoutWithResponse(t.Context())
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode())

	loggedOut := outboxEvents(t, "UserLoggedOut", func() *authevents.UserLoggedOut { return &authevents.UserLoggedOut{} })
	require.Contains(t, sessionIDs(loggedOut), session.SessionID)
}

func TestChangePassword(t *testing.T) {
	user := register(t)
	current := loginToken(t, user)
	other := loginToken(t, user)
	currentSession, ok := validateSession(t, current)
	require.True(t, ok)
	client := authClient(t)

	change := func(token, oldPassword, newPassword string) *publicapi.ChangePasswordResponse {
		t.Helper()
		var editors []publicapi.RequestEditorFn
		if token != "" {
			editors = append(editors, bearer(token))
		}
		response, err := client.ChangePasswordWithResponse(t.Context(), publicapi.ChangePasswordJSONRequestBody{
			OldPassword: oldPassword,
			NewPassword: newPassword,
		}, editors...)
		require.NoError(t, err)
		return response
	}

	require.Equal(t, http.StatusUnauthorized, change("", user.password, "new-password2").StatusCode())
	require.Equal(t, http.StatusUnauthorized, change("invalid-token", user.password, "new-password2").StatusCode())
	require.Equal(t, http.StatusForbidden, change(current, "wrong-password1", "new-password2").StatusCode())
	require.Equal(t, http.StatusBadRequest, change(current, user.password, "weak").StatusCode())
	require.Equal(t, http.StatusBadRequest, change(current, user.password, user.password).StatusCode())

	response := change(current, user.password, "new-password2")
	require.Equal(t, http.StatusNoContent, response.StatusCode(), string(response.Body))

	_, ok = validateSession(t, current)
	require.True(t, ok)
	_, ok = validateSession(t, other)
	require.False(t, ok)
	_, ok = validateSession(t, user.token)
	require.False(t, ok)

	require.Equal(t, http.StatusUnauthorized, login(t, user.login, user.password).StatusCode())
	require.Equal(t, http.StatusOK, login(t, user.login, "new-password2").StatusCode())
	require.Equal(t, http.StatusForbidden, change(current, user.password, "new-password3").StatusCode(),
		"the old password no longer authorizes a change")

	changed := outboxEvents(t, "PasswordChanged", func() *authevents.PasswordChanged { return &authevents.PasswordChanged{} })
	var found bool
	for _, e := range changed {
		if e.UserID == user.userID {
			found = true
			require.Equal(t, currentSession.SessionID, e.KeepSessionID)
			require.Positive(t, e.ChangedAt)
		}
	}
	require.True(t, found, "PasswordChanged must be stored in the outbox")
}

func registerBody(login, password, email string) publicapi.RegisterJSONRequestBody {
	return publicapi.RegisterJSONRequestBody{
		Login:    login,
		Password: password,
		Profile: publicapi.Profile{
			FirstName: "Иван",
			LastName:  "Петров",
			Email:     email,
		},
	}
}

func userIDs(events []*authevents.UserRegistered) []string {
	ids := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.UserID)
	}
	return ids
}

func sessionIDs[T interface{ GetSessionID() string }](events []T) []string {
	ids := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.GetSessionID())
	}
	return ids
}
