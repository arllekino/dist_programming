package integration_tests

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"

	publicapi "auth/api/server/publicapi"
)

func gatewayClient(t *testing.T) *publicapi.ClientWithResponses {
	t.Helper()
	client, err := publicapi.NewClientWithResponses("http://" + testEnv.gatewayAddress)
	require.NoError(t, err)
	return client
}

func currentUser(t *testing.T, headers map[string]string) (status int, body map[string]any) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		"http://"+testEnv.gatewayAddress+"/api/public/v1/users/me", http.NoBody)
	require.NoError(t, err)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, response.Body.Close())
	}()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &body), string(raw))
	}
	return response.StatusCode, body
}

func withToken(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func TestGatewayScenario(t *testing.T) {
	client := gatewayClient(t)
	login := "gw-" + uuid.New().String()[:8]

	registered, err := client.RegisterWithResponse(t.Context(), registerBody(login, defaultPassword, login+"@example.com"))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, registered.StatusCode(), string(registered.Body))
	userID := registered.JSON201.UserID

	loggedIn, err := client.LoginWithResponse(t.Context(), publicapi.LoginJSONRequestBody{Login: login, Password: defaultPassword})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, loggedIn.StatusCode())
	token := loggedIn.JSON200.Token
	other := registered.JSON201.Session.Token

	status, user := currentUser(t, withToken(token))
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, userID, user["id"])

	status, body := currentUser(t, nil)
	require.Equal(t, http.StatusUnauthorized, status)
	require.Equal(t, "unauthenticated", body["message"])

	status, _ = currentUser(t, map[string]string{"X-Auth-User": userID})
	require.Equal(t, http.StatusUnauthorized, status, "a client must not be able to impersonate a user")

	changed, err := client.ChangePasswordWithResponse(t.Context(), publicapi.ChangePasswordJSONRequestBody{
		OldPassword: defaultPassword,
		NewPassword: "password2",
	}, bearer(token))
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, changed.StatusCode(), string(changed.Body))

	status, _ = currentUser(t, withToken(other))
	require.Equal(t, http.StatusUnauthorized, status)

	loggedOut, err := client.LogoutWithResponse(t.Context(), bearer(token))
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, loggedOut.StatusCode(), string(loggedOut.Body))

	status, _ = currentUser(t, withToken(token))
	require.Equal(t, http.StatusUnauthorized, status)
}

func TestGatewayRouting(t *testing.T) {
	for _, tt := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodGet, path: "/api/public/v1/login", status: http.StatusForbidden},
		{method: http.MethodDelete, path: "/api/public/v1/users/me", status: http.StatusForbidden},
		{method: http.MethodGet, path: "/api/public/v1/unknown", status: http.StatusNotFound},
		{method: http.MethodPost, path: "/api/public/v1/logout", status: http.StatusUnauthorized},
	} {
		request, err := http.NewRequestWithContext(t.Context(), tt.method, "http://"+testEnv.gatewayAddress+tt.path, http.NoBody)
		require.NoError(t, err)
		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, tt.status, response.StatusCode, "%s %s", tt.method, tt.path)
	}
}
