package clients

import (
	"context"
	"time"
	"uuid"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sessionapi "auth/api/clients/sessionapi"
	"auth/internal/auth/app"
)

const sessionService = "session"

func NewSessionClient(connection grpc.ClientConnInterface, timeout time.Duration) app.SessionClient {
	return &sessionClient{
		client:  sessionapi.NewSessionServiceClient(connection),
		timeout: timeout,
	}
}

type sessionClient struct {
	client  sessionapi.SessionServiceClient
	timeout time.Duration
}

func (c *sessionClient) Create(ctx context.Context, userID uuid.UUID) (app.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.client.CreateSession(ctx, &sessionapi.CreateSessionRequest{
		UserID: userID.String(),
	})
	if err != nil {
		return app.Session{}, unavailable(sessionService, err)
	}
	return app.Session{
		Token:     response.Token,
		ID:        response.SessionID,
		UserID:    userID,
		ExpiresAt: time.UnixMilli(response.ExpiresAt).UTC(),
	}, nil
}

func (c *sessionClient) Validate(ctx context.Context, token string) (app.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.client.ValidateSession(ctx, &sessionapi.ValidateSessionRequest{Token: token})
	if status.Code(err) == codes.Unauthenticated {
		return app.Session{}, app.ErrUnauthenticated
	}
	if err != nil {
		return app.Session{}, unavailable(sessionService, err)
	}
	return toSession(token, response.UserID, response.SessionID, time.UnixMilli(response.ExpiresAt))
}

func (c *sessionClient) Revoke(ctx context.Context, token string) (app.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.client.RevokeSession(ctx, &sessionapi.RevokeSessionRequest{Token: token})
	if status.Code(err) == codes.NotFound {
		return app.Session{}, app.ErrUnauthenticated
	}
	if err != nil {
		return app.Session{}, unavailable(sessionService, err)
	}
	return toSession(token, response.UserID, response.SessionID, time.Time{})
}

func (c *sessionClient) RevokeUserSessions(ctx context.Context, userID uuid.UUID, exceptSessionID string, createdBefore time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_, err := c.client.RevokeUserSessions(ctx, &sessionapi.RevokeUserSessionsRequest{
		UserID:          userID.String(),
		ExceptSessionID: exceptSessionID,
		CreatedBefore:   createdBefore.UnixMilli(),
	})
	if err != nil {
		return unavailable(sessionService, err)
	}
	return nil
}

func toSession(token, rawUserID, sessionID string, expiresAt time.Time) (app.Session, error) {
	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		return app.Session{}, unavailable(sessionService, err)
	}
	return app.Session{
		Token:     token,
		ID:        sessionID,
		UserID:    userID,
		ExpiresAt: expiresAt.UTC(),
	}, nil
}
