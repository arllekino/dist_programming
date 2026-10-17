package internalapi

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sessioninternalapi "session/api/server/internalapi"
	"session/internal/session/app"
	"session/internal/session/domain"
)

func NewHandler(svc *app.Service, logger *slog.Logger) sessioninternalapi.SessionServiceServer {
	return &handler{
		svc:    svc,
		logger: logger,
	}
}

type handler struct {
	svc    *app.Service
	logger *slog.Logger
	sessioninternalapi.UnimplementedSessionServiceServer
}

func (h handler) CreateSession(ctx context.Context, req *sessioninternalapi.CreateSessionRequest) (*sessioninternalapi.CreateSessionResponse, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}
	token, session, err := h.svc.CreateSession(ctx, userID, req.UserAgent)
	if err != nil {
		return nil, h.error(ctx, "create session", err)
	}
	return &sessioninternalapi.CreateSessionResponse{
		Token:     string(token),
		SessionID: session.ID,
		ExpiresAt: session.ExpiresAt.UnixMilli(),
	}, nil
}

func (h handler) ValidateSession(ctx context.Context, req *sessioninternalapi.ValidateSessionRequest) (*sessioninternalapi.ValidateSessionResponse, error) {
	session, err := h.svc.ValidateSession(ctx, req.Token)
	if errors.Is(err, domain.ErrSessionNotFound) {
		return nil, status.Error(codes.Unauthenticated, "invalid session")
	}
	if err != nil {
		return nil, h.error(ctx, "validate session", err)
	}
	return &sessioninternalapi.ValidateSessionResponse{
		UserID:    session.UserID.String(),
		SessionID: session.ID,
		ExpiresAt: session.ExpiresAt.UnixMilli(),
	}, nil
}

func (h handler) RevokeSession(ctx context.Context, req *sessioninternalapi.RevokeSessionRequest) (*sessioninternalapi.RevokeSessionResponse, error) {
	session, err := h.svc.RevokeSession(ctx, req.Token)
	if errors.Is(err, domain.ErrSessionNotFound) {
		return nil, status.Error(codes.NotFound, "session not found")
	}
	if err != nil {
		return nil, h.error(ctx, "revoke session", err)
	}
	return &sessioninternalapi.RevokeSessionResponse{
		UserID:    session.UserID.String(),
		SessionID: session.ID,
	}, nil
}

func (h handler) RevokeUserSessions(ctx context.Context, req *sessioninternalapi.RevokeUserSessionsRequest) (*sessioninternalapi.RevokeUserSessionsResponse, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}
	var createdBefore time.Time
	if req.CreatedBefore != 0 {
		createdBefore = time.UnixMilli(req.CreatedBefore)
	}
	revoked, err := h.svc.RevokeUserSessions(ctx, userID, req.ExceptSessionID, createdBefore)
	if err != nil {
		return nil, h.error(ctx, "revoke user sessions", err)
	}
	return &sessioninternalapi.RevokeUserSessionsResponse{
		Revoked: int32(revoked), //nolint:gosec
	}, nil
}

func (h handler) error(ctx context.Context, operation string, err error) error {
	if errors.Is(err, domain.ErrInvalidUserID) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	h.logger.ErrorContext(ctx, operation+" failed", logging.Error(err))
	return status.Error(codes.Internal, operation+" failed")
}
