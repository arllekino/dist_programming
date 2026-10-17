package integration_tests

import (
	"bytes"
	"net/http"
	"testing"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/event"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	identityevents "identity/api/events"
	internalapi "identity/api/server/internalapi"
	publicapi "identity/api/server/publicapi"
)

const (
	petrFirstName = "Петр"
	petrLastName  = "Иванов"
	petrEmail     = "petr@example.com"
)

func validProfile() *internalapi.Profile {
	return &internalapi.Profile{FirstName: "Иван", LastName: "Петров", Email: "Ivan@Example.com"}
}

func TestCreateAndGetUser(t *testing.T) {
	client := identityClient(t)
	userID := uuid.New().String()

	created, err := client.CreateUser(callContext(t), &internalapi.CreateUserRequest{
		UserID:  userID,
		Profile: validProfile(),
	}, grpc.WaitForReady(true))
	require.NoError(t, err)
	require.Equal(t, userID, created.UserID)
	require.Equal(t, "ivan@example.com", created.Profile.Email)
	require.Equal(t, "active", created.Status)

	retried, err := client.CreateUser(callContext(t), &internalapi.CreateUserRequest{
		UserID:  userID,
		Profile: validProfile(),
	})
	require.NoError(t, err, "a retry from Auth must be idempotent")
	require.Equal(t, created.UserID, retried.UserID)

	_, err = client.CreateUser(callContext(t), &internalapi.CreateUserRequest{
		UserID:  userID,
		Profile: &internalapi.Profile{FirstName: petrFirstName, LastName: petrLastName, Email: petrEmail},
	})
	requireCode(t, codes.AlreadyExists, err)

	found, err := client.GetUser(callContext(t), &internalapi.GetUserRequest{UserID: userID})
	require.NoError(t, err)
	require.Equal(t, "Иван", found.Profile.FirstName)

	_, err = client.GetUser(callContext(t), &internalapi.GetUserRequest{UserID: uuid.New().String()})
	requireCode(t, codes.NotFound, err)

	created1 := requireUserEvents(t, "UserCreated", userID)
	require.Len(t, created1, 1, "the idempotent retry must not publish a second event")
}

func TestCreateUserValidation(t *testing.T) {
	client := identityClient(t)

	for _, req := range []*internalapi.CreateUserRequest{
		{UserID: "not-a-uuid", Profile: validProfile()},
		{UserID: uuid.Nil().String(), Profile: validProfile()},
		{UserID: uuid.New().String()},
		{UserID: uuid.New().String(), Profile: &internalapi.Profile{FirstName: "Иван", LastName: "Петров", Email: "broken"}},
	} {
		_, err := client.CreateUser(callContext(t), req, grpc.WaitForReady(true))
		requireCode(t, codes.InvalidArgument, err)
	}
}

func TestCurrentUserHTTP(t *testing.T) {
	client := identityClient(t)
	userID := uuid.New()
	_, err := client.CreateUser(callContext(t), &internalapi.CreateUserRequest{
		UserID:  userID.String(),
		Profile: validProfile(),
	}, grpc.WaitForReady(true))
	require.NoError(t, err)

	httpClient, err := publicapi.NewClientWithResponses("http://" + testEnv.conf.HTTP.Address)
	require.NoError(t, err)

	current, err := httpClient.GetCurrentUserWithResponse(t.Context(), &publicapi.GetCurrentUserParams{XAuthUser: publicapi.AuthUser(userID)})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, current.StatusCode())
	require.Equal(t, userID.String(), current.JSON200.Id)
	require.Equal(t, "Петров", current.JSON200.Profile.LastName)

	updated, err := httpClient.UpdateCurrentUserWithResponse(t.Context(),
		&publicapi.UpdateCurrentUserParams{XAuthUser: publicapi.AuthUser(userID)},
		publicapi.UpdateCurrentUserJSONRequestBody{FirstName: petrFirstName, LastName: petrLastName, Email: petrEmail},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, updated.StatusCode())
	require.Equal(t, petrFirstName, updated.JSON200.Profile.FirstName)
	require.Len(t, requireUserEvents(t, "UserProfileUpdated", userID.String()), 1)

	invalid, err := httpClient.UpdateCurrentUserWithResponse(t.Context(),
		&publicapi.UpdateCurrentUserParams{XAuthUser: publicapi.AuthUser(userID)},
		publicapi.UpdateCurrentUserJSONRequestBody{FirstName: " ", LastName: petrLastName, Email: petrEmail},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, invalid.StatusCode())
	require.NotEmpty(t, invalid.JSON400.Message)

	missing, err := httpClient.GetCurrentUserWithResponse(t.Context(), &publicapi.GetCurrentUserParams{XAuthUser: publicapi.AuthUser(uuid.New())})
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, missing.StatusCode())
}

func TestCurrentUserRequiresUserIDHeader(t *testing.T) {
	for _, header := range []string{"", "not-a-uuid"} {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
			"http://"+testEnv.conf.HTTP.Address+"/api/public/v1/users/me", bytes.NewReader(nil))
		require.NoError(t, err)
		if header != "" {
			request.Header.Set("X-Auth-User", header)
		}
		response, err := http.DefaultClient.Do(request)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusBadRequest, response.StatusCode)
	}
}

type userEvent interface {
	proto.Message
	GetUserID() string
}

var userEventPayloads = map[string]func() userEvent{
	"UserCreated":        func() userEvent { return &identityevents.UserCreated{} },
	"UserProfileUpdated": func() userEvent { return &identityevents.UserProfileUpdated{} },
}

func requireUserEvents(t *testing.T, eventType, userID string) []event.Envelope {
	t.Helper()
	var matched []event.Envelope
	for _, envelope := range outboxEvents(t, eventType) {
		payload := userEventPayloads[eventType]()
		require.NoError(t, envelope.UnmarshalPayload(payload))
		if payload.GetUserID() == userID {
			matched = append(matched, envelope)
		}
	}
	return matched
}
