package internalapi

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityinternalapi "identity/api/server/internalapi"
	"identity/internal/identity/app"
	"identity/internal/identity/domain"
)

func NewHandler(svc *app.Service, logger *slog.Logger) identityinternalapi.IdentityServiceServer {
	return &handler{
		svc:    svc,
		logger: logger,
	}
}

type handler struct {
	svc    *app.Service
	logger *slog.Logger
	identityinternalapi.UnimplementedIdentityServiceServer
}

func (h handler) CreateUser(ctx context.Context, req *identityinternalapi.CreateUserRequest) (*identityinternalapi.User, error) {
	id, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}
	profile, err := domain.NewProfile(req.GetProfile().GetFirstName(), req.GetProfile().GetLastName(), req.GetProfile().GetEmail())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	user, err := h.svc.CreateUser(ctx, id, profile)
	if err != nil {
		return nil, h.error(ctx, "create user", err)
	}
	return convertToUser(user), nil
}

func (h handler) GetUser(ctx context.Context, req *identityinternalapi.GetUserRequest) (*identityinternalapi.User, error) {
	id, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}
	user, err := h.svc.GetUser(ctx, id)
	if err != nil {
		return nil, h.error(ctx, "get user", err)
	}
	return convertToUser(user), nil
}

func (h handler) error(ctx context.Context, operation string, err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidUserID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrUserAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	}
	h.logger.ErrorContext(ctx, operation+" failed", logging.Error(err))
	return status.Error(codes.Internal, operation+" failed")
}

func convertToUser(user domain.User) *identityinternalapi.User {
	return &identityinternalapi.User{
		UserID: user.ID.String(),
		Profile: &identityinternalapi.Profile{
			FirstName: user.Profile.FirstName,
			LastName:  user.Profile.LastName,
			Email:     user.Profile.Email,
		},
		Status: string(user.Status),
	}
}
