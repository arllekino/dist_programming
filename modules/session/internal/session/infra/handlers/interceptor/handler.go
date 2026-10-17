package interceptor

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/distributed-programming-2026/go-sdk/pkg/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"session/api/server/interceptorgrpc"
	"session/internal/session/app"
	"session/internal/session/domain"
)

const (
	HeaderAuthorization = "Authorization"
	HeaderUser          = "X-Auth-User"
	HeaderSession       = "X-Auth-Session"

	ParameterKeepAuthorization = "keepAuthorization"
)

var unauthenticatedBody = []byte(`{"message":"unauthenticated"}`)

func NewHandler(svc *app.Service, logger *slog.Logger) interceptorgrpc.InterceptorServer {
	return &handler{
		svc:    svc,
		logger: logger,
	}
}

type handler struct {
	svc    *app.Service
	logger *slog.Logger
	interceptorgrpc.UnimplementedInterceptorServer
}

func (h handler) Intercept(ctx context.Context, req *interceptorgrpc.InterceptRequest) (*interceptorgrpc.InterceptResponse, error) {
	headers := req.GetRequestHeaders()
	if headers == nil {
		return continueResponse(nil), nil
	}

	session, err := h.svc.ValidateSession(ctx, bearerToken(headers.GetHeaders()))
	if errors.Is(err, domain.ErrSessionNotFound) {
		return unauthenticated(), nil
	}
	if err != nil {
		h.logger.ErrorContext(ctx, "intercept failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "session validation failed")
	}

	mutation := &interceptorgrpc.HeaderMutation{
		Set: []*interceptorgrpc.HeaderValues{
			{Key: HeaderUser, Values: []string{session.UserID.String()}},
			{Key: HeaderSession, Values: []string{session.ID}},
		},
	}
	if req.GetContext().GetParameters()[ParameterKeepAuthorization] != "true" {
		mutation.Remove = []string{HeaderAuthorization}
	}
	return continueResponse(mutation), nil
}

func bearerToken(headers []*interceptorgrpc.Header) string {
	for _, header := range headers {
		if !strings.EqualFold(header.GetKey(), HeaderAuthorization) {
			continue
		}
		scheme, token, found := strings.Cut(header.GetValue(), " ")
		if found && strings.EqualFold(scheme, "Bearer") {
			return strings.TrimSpace(token)
		}
	}
	return ""
}

func continueResponse(mutation *interceptorgrpc.HeaderMutation) *interceptorgrpc.InterceptResponse {
	return &interceptorgrpc.InterceptResponse{
		Result: &interceptorgrpc.InterceptResponse_ContinueResponse{
			ContinueResponse: &interceptorgrpc.ContinueResponse{
				HeaderMutation: mutation,
			},
		},
	}
}

func unauthenticated() *interceptorgrpc.InterceptResponse {
	return &interceptorgrpc.InterceptResponse{
		Result: &interceptorgrpc.InterceptResponse_ImmediateResponse{
			ImmediateResponse: &interceptorgrpc.ImmediateResponse{
				StatusCode: 401,
				Headers: []*interceptorgrpc.Header{
					{Key: "Content-Type", Value: "application/json"},
					{Key: "WWW-Authenticate", Value: "Bearer"},
				},
				Body: unauthenticatedBody,
			},
		},
	}
}
