package public

import (
	"context"
	"errors"
	"uuid"

	publicapi "identity/api/server/publicapi"
	"identity/internal/identity/app"
	"identity/internal/identity/domain"
)

func NewHandler(svc *app.Service) publicapi.StrictServerInterface {
	return &handler{
		svc: svc,
	}
}

type handler struct {
	svc *app.Service
}

func (h handler) GetCurrentUser(ctx context.Context, req publicapi.GetCurrentUserRequestObject) (publicapi.GetCurrentUserResponseObject, error) {
	user, err := h.svc.GetUser(ctx, uuid.UUID(req.Params.XAuthUser))
	if errors.Is(err, domain.ErrUserNotFound) {
		return publicapi.GetCurrentUser404Response{}, nil
	}
	if err != nil {
		return nil, err
	}
	return publicapi.GetCurrentUser200JSONResponse(convertToUser(user)), nil
}

func (h handler) UpdateCurrentUser(ctx context.Context, req publicapi.UpdateCurrentUserRequestObject) (publicapi.UpdateCurrentUserResponseObject, error) {
	profile, err := domain.NewProfile(req.Body.FirstName, req.Body.LastName, req.Body.Email)
	if err != nil {
		return publicapi.UpdateCurrentUser400JSONResponse{Message: err.Error()}, nil
	}
	user, err := h.svc.UpdateProfile(ctx, uuid.UUID(req.Params.XAuthUser), profile)
	if errors.Is(err, domain.ErrUserNotFound) {
		return publicapi.UpdateCurrentUser404Response{}, nil
	}
	if err != nil {
		return nil, err
	}
	return publicapi.UpdateCurrentUser200JSONResponse(convertToUser(user)), nil
}

func convertToUser(user domain.User) publicapi.User {
	return publicapi.User{
		Id: user.ID.String(),
		Profile: publicapi.Profile{
			FirstName: user.Profile.FirstName,
			LastName:  user.Profile.LastName,
			Email:     user.Profile.Email,
		},
		Status: publicapi.UserStatus(user.Status),
	}
}
