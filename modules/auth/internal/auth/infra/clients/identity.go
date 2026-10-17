package clients

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityapi "auth/api/clients/identityapi"
	"auth/internal/auth/app"
)

func NewIdentityClient(connection grpc.ClientConnInterface, timeout time.Duration) app.IdentityClient {
	return &identityClient{
		client:  identityapi.NewIdentityServiceClient(connection),
		timeout: timeout,
	}
}

type identityClient struct {
	client  identityapi.IdentityServiceClient
	timeout time.Duration
}

func (c *identityClient) CreateUser(ctx context.Context, userID uuid.UUID, profile app.Profile) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_, err := c.client.CreateUser(ctx, &identityapi.CreateUserRequest{
		UserID: userID.String(),
		Profile: &identityapi.Profile{
			FirstName: profile.FirstName,
			LastName:  profile.LastName,
			Email:     profile.Email,
		},
	})
	if s, ok := status.FromError(err); ok && s.Code() == codes.InvalidArgument {
		return fmt.Errorf("%w: %s", app.ErrInvalidProfile, s.Message())
	}
	if err != nil {
		return unavailable("identity", err)
	}
	return nil
}
