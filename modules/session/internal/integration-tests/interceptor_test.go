package integration_tests

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"session/api/server/interceptorgrpc"
	internalapi "session/api/server/internalapi"
)

func interceptorClient(t *testing.T) interceptorgrpc.InterceptorClient {
	t.Helper()
	return interceptorgrpc.NewInterceptorClient(grpcConnection(t))
}

func requestHeaders(parameters map[string]string, headers ...*interceptorgrpc.Header) *interceptorgrpc.InterceptRequest {
	return &interceptorgrpc.InterceptRequest{
		Context: &interceptorgrpc.ProcessingContext{
			RequestId: uuid.New().String(),
			Request: &interceptorgrpc.HttpRequest{
				Method:  "GET",
				Path:    "/api/public/v1/users/me",
				Headers: headers,
			},
			Parameters: parameters,
		},
		Event: &interceptorgrpc.InterceptRequest_RequestHeaders{
			RequestHeaders: &interceptorgrpc.HttpHeaders{
				Headers:     headers,
				EndOfStream: true,
			},
		},
	}
}

func authorization(value string) *interceptorgrpc.Header {
	return &interceptorgrpc.Header{Key: "authorization", Value: value}
}

func TestInterceptorAuthenticatesRequest(t *testing.T) {
	client := interceptorClient(t)
	userID := uuid.New().String()
	session := createSession(t, sessionClient(t), userID)

	response, err := client.Intercept(callContext(t), requestHeaders(nil, authorization("Bearer "+session.Token)), grpc.WaitForReady(true))
	require.NoError(t, err)
	mutation := response.GetContinueResponse().GetHeaderMutation()
	require.NotNil(t, mutation)
	require.Equal(t, []string{"Authorization"}, mutation.Remove)
	require.Len(t, mutation.Set, 2)
	require.Equal(t, "X-Auth-User", mutation.Set[0].Key)
	require.Equal(t, []string{userID}, mutation.Set[0].Values)
	require.Equal(t, "X-Auth-Session", mutation.Set[1].Key)
	require.Equal(t, []string{session.SessionID}, mutation.Set[1].Values)

	kept, err := client.Intercept(callContext(t), requestHeaders(
		map[string]string{"keepAuthorization": "true"},
		authorization("bearer "+session.Token),
	))
	require.NoError(t, err)
	require.Empty(t, kept.GetContinueResponse().GetHeaderMutation().GetRemove())
	require.Len(t, kept.GetContinueResponse().GetHeaderMutation().GetSet(), 2)
}

func TestInterceptorRejectsRequest(t *testing.T) {
	client := interceptorClient(t)
	sessions := sessionClient(t)
	revoked := createSession(t, sessions, uuid.New().String())
	_, err := sessions.RevokeSession(callContext(t), &internalapi.RevokeSessionRequest{Token: revoked.Token})
	require.NoError(t, err)

	for name, headers := range map[string][]*interceptorgrpc.Header{
		"no authorization": nil,
		"basic scheme":     {authorization("Basic dXNlcjpwYXNz")},
		"malformed token":  {authorization("Bearer malformed")},
		"revoked session":  {authorization("Bearer " + revoked.Token)},
	} {
		t.Run(name, func(t *testing.T) {
			response, err := client.Intercept(callContext(t), requestHeaders(nil, headers...), grpc.WaitForReady(true))
			require.NoError(t, err)
			immediate := response.GetImmediateResponse()
			require.NotNil(t, immediate)
			require.EqualValues(t, 401, immediate.StatusCode)
			require.JSONEq(t, `{"message":"unauthenticated"}`, string(immediate.Body))
		})
	}
}

func TestInterceptorIgnoresOtherPhases(t *testing.T) {
	response, err := interceptorClient(t).Intercept(callContext(t), &interceptorgrpc.InterceptRequest{
		Context: &interceptorgrpc.ProcessingContext{RequestId: uuid.New().String()},
		Event: &interceptorgrpc.InterceptRequest_ResponseHeaders{
			ResponseHeaders: &interceptorgrpc.HttpHeaders{},
		},
	}, grpc.WaitForReady(true))
	require.NoError(t, err)
	require.NotNil(t, response.GetContinueResponse())
	require.Nil(t, response.GetContinueResponse().GetHeaderMutation())
}
