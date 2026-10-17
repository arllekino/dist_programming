package integration_tests

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/event"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	identityapi "auth/api/clients/identityapi"
	sessionapi "auth/api/clients/sessionapi"
	publicapi "auth/api/server/publicapi"
)

type logBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *logBuffer) Accept(log testcontainers.Log) {
	_, _ = b.Write(log.Content)
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func authClient(t *testing.T) *publicapi.ClientWithResponses {
	t.Helper()
	client, err := publicapi.NewClientWithResponses("http://" + testEnv.conf.HTTP.Address)
	require.NoError(t, err)
	return client
}

func bearer(token string) publicapi.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}

func grpcConnection(t *testing.T, address string) *grpc.ClientConn {
	t.Helper()
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, connection.Close())
	})
	return connection
}

func callContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func validateSession(t *testing.T, token string) (*sessionapi.ValidateSessionResponse, bool) {
	t.Helper()
	client := sessionapi.NewSessionServiceClient(grpcConnection(t, testEnv.sessionAddress))
	response, err := client.ValidateSession(callContext(t), &sessionapi.ValidateSessionRequest{Token: token})
	if status.Code(err) == codes.Unauthenticated {
		return nil, false
	}
	require.NoError(t, err)
	return response, true
}

func getIdentityUser(t *testing.T, userID string) *identityapi.User {
	t.Helper()
	client := identityapi.NewIdentityServiceClient(grpcConnection(t, testEnv.identityAddress))
	user, err := client.GetUser(callContext(t), &identityapi.GetUserRequest{UserID: userID})
	require.NoError(t, err)
	return user
}

const defaultPassword = "password1"

type testUser struct {
	login    string
	password string
	userID   string
	token    string
}

func register(t *testing.T) testUser {
	t.Helper()
	user := testUser{
		login:    "user-" + uuid.New().String()[:8],
		password: defaultPassword,
	}
	response, err := authClient(t).RegisterWithResponse(t.Context(), publicapi.RegisterJSONRequestBody{
		Login:    user.login,
		Password: user.password,
		Profile: publicapi.Profile{
			FirstName: "Иван",
			LastName:  "Петров",
			Email:     user.login + "@example.com",
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, response.StatusCode(), string(response.Body))
	require.NotNil(t, response.JSON201.Session)
	user.userID = response.JSON201.UserID
	user.token = response.JSON201.Session.Token
	return user
}

func login(t *testing.T, login, password string) *publicapi.LoginResponse {
	t.Helper()
	response, err := authClient(t).LoginWithResponse(t.Context(), publicapi.LoginJSONRequestBody{
		Login:    login,
		Password: password,
	})
	require.NoError(t, err)
	return response
}

func loginToken(t *testing.T, user testUser) string {
	t.Helper()
	response := login(t, user.login, user.password)
	require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
	return response.JSON200.Token
}

func outboxEvents[T proto.Message](t *testing.T, eventType string, newPayload func() T) []T {
	t.Helper()
	const query = `
		SELECT
			payload
		FROM
			event_outbox
	`
	var rows []struct {
		Payload []byte `db:"payload"`
	}
	err := testEnv.connector.TransactionalClient().SelectContext(t.Context(), "test.outbox", &rows, query)
	require.NoError(t, err)
	var payloads []T
	for _, row := range rows {
		envelope, err := event.Unmarshal(row.Payload)
		require.NoError(t, err)
		if envelope.Type() != eventType {
			continue
		}
		require.Equal(t, moduleID, envelope.Producer())
		payload := newPayload()
		require.NoError(t, envelope.UnmarshalPayload(payload))
		payloads = append(payloads, payload)
	}
	return payloads
}
