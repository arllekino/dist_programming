package integration_tests

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/distributed-programming-2026/go-sdk/pkg/event"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	internalapi "identity/api/server/internalapi"
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

func identityClient(t *testing.T) internalapi.IdentityServiceClient {
	t.Helper()
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, connection.Close())
	})
	return internalapi.NewIdentityServiceClient(connection)
}

func callContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func requireCode(t *testing.T, code codes.Code, err error) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, code, status.Code(err), err.Error())
}

func outboxEvents(t *testing.T, eventType string) []event.Envelope {
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
	var envelopes []event.Envelope
	for _, row := range rows {
		envelope, err := event.Unmarshal(row.Payload)
		require.NoError(t, err)
		if envelope.Type() == eventType {
			require.Equal(t, moduleID, envelope.Producer())
			envelopes = append(envelopes, envelope)
		}
	}
	return envelopes
}
