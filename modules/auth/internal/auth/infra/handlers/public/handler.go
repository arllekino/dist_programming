package public

import (
	"context"
	"errors"

	publicapi "auth/api/server/publicapi"
	"auth/internal/auth/app"
	"auth/internal/auth/domain"
)

func NewHandler(svc *app.Service) publicapi.StrictServerInterface {
	return &handler{
		svc: svc,
	}
}

type handler struct {
	svc *app.Service
}

func (h handler) Register(ctx context.Context, req publicapi.RegisterRequestObject) (publicapi.RegisterResponseObject, error) {
	profile := app.Profile{
		FirstName: req.Body.Profile.FirstName,
		LastName:  req.Body.Profile.LastName,
		Email:     req.Body.Profile.Email,
	}
	result, err := h.svc.Register(ctx, req.Body.Login, req.Body.Password, profile, metaFrom(ctx).userAgent)
	switch {
	case isValidationError(err):
		return publicapi.Register400JSONResponse(errorBody(err)), nil
	case errors.Is(err, domain.ErrLoginTaken):
		return publicapi.Register409JSONResponse(errorBody(err)), nil
	case errors.Is(err, app.ErrDependencyUnavailable):
		return publicapi.Register503JSONResponse(unavailableBody()), nil
	case err != nil:
		return nil, err
	}

	response := publicapi.Register201JSONResponse{UserID: result.UserID.String()}
	if result.Session != nil {
		session := toSession(*result.Session)
		response.Session = &session
	}
	return response, nil
}

func (h handler) Login(ctx context.Context, req publicapi.LoginRequestObject) (publicapi.LoginResponseObject, error) {
	session, err := h.svc.Login(ctx, req.Body.Login, req.Body.Password, metaFrom(ctx).userAgent)
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		return publicapi.Login401JSONResponse(errorBody(domain.ErrInvalidCredentials)), nil
	case errors.Is(err, app.ErrDependencyUnavailable):
		return publicapi.Login503JSONResponse(unavailableBody()), nil
	case err != nil:
		return nil, err
	}
	return publicapi.Login200JSONResponse(toSession(session)), nil
}

func (h handler) Logout(ctx context.Context, _ publicapi.LogoutRequestObject) (publicapi.LogoutResponseObject, error) {
	err := h.svc.Logout(ctx, metaFrom(ctx).token)
	switch {
	case errors.Is(err, app.ErrUnauthenticated):
		return publicapi.Logout401JSONResponse(errorBody(err)), nil
	case errors.Is(err, app.ErrDependencyUnavailable):
		return publicapi.Logout503JSONResponse(unavailableBody()), nil
	case err != nil:
		return nil, err
	}
	return publicapi.Logout204Response{}, nil
}

func (h handler) ChangePassword(ctx context.Context, req publicapi.ChangePasswordRequestObject) (publicapi.ChangePasswordResponseObject, error) {
	err := h.svc.ChangePassword(ctx, metaFrom(ctx).token, req.Body.OldPassword, req.Body.NewPassword)
	switch {
	case errors.Is(err, app.ErrUnauthenticated):
		return publicapi.ChangePassword401JSONResponse(errorBody(err)), nil
	case errors.Is(err, domain.ErrWrongPassword):
		return publicapi.ChangePassword403JSONResponse(errorBody(err)), nil
	case isValidationError(err):
		return publicapi.ChangePassword400JSONResponse(errorBody(err)), nil
	case errors.Is(err, app.ErrDependencyUnavailable):
		return publicapi.ChangePassword503JSONResponse(unavailableBody()), nil
	case err != nil:
		return nil, err
	}
	return publicapi.ChangePassword204Response{}, nil
}

func isValidationError(err error) bool {
	return errors.Is(err, domain.ErrInvalidLogin) ||
		errors.Is(err, domain.ErrInvalidPassword) ||
		errors.Is(err, domain.ErrSamePassword) ||
		errors.Is(err, app.ErrInvalidProfile)
}

func errorBody(err error) publicapi.Error {
	return publicapi.Error{Message: err.Error()}
}

func unavailableBody() publicapi.Error {
	return publicapi.Error{Message: "service temporarily unavailable"}
}

func toSession(session app.Session) publicapi.Session {
	return publicapi.Session{
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt,
	}
}
