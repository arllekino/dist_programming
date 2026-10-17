package integration_tests

import (
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/amqp"
	"github.com/distributed-programming-2026/go-sdk/pkg/event"
	eventamqp "github.com/distributed-programming-2026/go-sdk/pkg/event/amqp"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	domainevent "github.com/distributed-programming-2026/lib/event"

	internalapi "session/api/server/internalapi"
	authevents "session/api/subscriptions/authevents"
)

func TestPasswordChangedRevokesOlderSessions(t *testing.T) {
	client := sessionClient(t)
	userID := uuid.New().String()

	old := createSession(t, client, userID)
	kept := createSession(t, client, userID)
	changedAt := time.UnixMilli(kept.ExpiresAt).Add(-idleTTL)
	time.Sleep(10 * time.Millisecond)
	later := createSession(t, client, userID)

	envelope, err := event.New(t.Context(), "auth", "PasswordChanged", &authevents.PasswordChanged{
		UserID:        userID,
		ChangedAt:     changedAt.UnixMilli(),
		KeepSessionID: kept.SessionID,
	})
	require.NoError(t, err)

	connection := amqp.NewConnection(moduleID+"-test", amqp.ConnectionConfig{
		URL:            testEnv.conf.AMQP.URL,
		ConnectTimeout: time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer func() {
		require.NoError(t, connection.Stop())
	}()
	dispatcher, err := eventamqp.NewDispatcher(connection.Producer(domainevent.DomainExchange(), nil, nil))
	require.NoError(t, err)
	require.NoError(t, connection.Start())

	for range 2 {
		require.NoError(t, dispatcher.Dispatch(t.Context(), envelope))
	}

	require.Eventually(t, func() bool {
		return testEnv.logs.count("event handling completed", "event_id", envelope.ID()) >= 2
	}, 10*time.Second, 100*time.Millisecond)

	_, err = client.ValidateSession(callContext(t), &internalapi.ValidateSessionRequest{Token: old.Token})
	requireCode(t, codes.Unauthenticated, err)
	requireValid(t, client, kept.Token, userID)
	requireValid(t, client, later.Token, userID)
	require.Equal(t, 2, testEnv.logs.count("sessions revoked after password change", "user_id", userID))
}
