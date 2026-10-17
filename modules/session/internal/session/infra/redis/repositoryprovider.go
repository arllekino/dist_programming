package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"

	"session/internal/session/domain"
)

func NewRepositoryProvider(client goredis.UniversalClient) *RepositoryProvider {
	return &RepositoryProvider{
		client: client,
	}
}

type RepositoryProvider struct {
	client goredis.UniversalClient
}

func (p *RepositoryProvider) Session(ctx context.Context) domain.SessionRepository {
	return NewSessionRepository(ctx, p.client)
}
