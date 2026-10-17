package integration_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	internalapi "session/api/server/internalapi"
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

func (b *logBuffer) count(message, key, value string) int {
	var count int
	for _, line := range strings.Split(b.String(), "\n") {
		var record map[string]any
		err := json.Unmarshal([]byte(line), &record)
		if err == nil && record["msg"] == message && (key == "" || record[key] == value) {
			count++
		}
	}
	return count
}

func grpcConnection(t *testing.T) *grpc.ClientConn {
	t.Helper()
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, connection.Close())
	})
	return connection
}

func sessionClient(t *testing.T) internalapi.SessionServiceClient {
	t.Helper()
	return internalapi.NewSessionServiceClient(grpcConnection(t))
}

func callContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func createSession(t *testing.T, client internalapi.SessionServiceClient, userID string) *internalapi.CreateSessionResponse {
	t.Helper()
	response, err := client.CreateSession(callContext(t), &internalapi.CreateSessionRequest{
		UserID: userID,
	}, grpc.WaitForReady(true))
	require.NoError(t, err)
	require.NotEmpty(t, response.Token)
	require.NotEmpty(t, response.SessionID)
	return response
}

func requireValid(t *testing.T, client internalapi.SessionServiceClient, token, userID string) *internalapi.ValidateSessionResponse {
	t.Helper()
	response, err := client.ValidateSession(callContext(t), &internalapi.ValidateSessionRequest{Token: token})
	require.NoError(t, err)
	require.Equal(t, userID, response.UserID)
	return response
}

func requireInvalid(t *testing.T, client internalapi.SessionServiceClient, token string) {
	t.Helper()
	_, err := client.ValidateSession(callContext(t), &internalapi.ValidateSessionRequest{Token: token})
	requireCode(t, codes.Unauthenticated, err)
}

func requireCode(t *testing.T, code codes.Code, err error) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, status.Code(err), err.Error())
}
