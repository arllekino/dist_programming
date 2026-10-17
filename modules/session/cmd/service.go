package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"

	"session/internal/session/app"
	"session/internal/session/config"
	"session/internal/session/domain"
	sessionredis "session/internal/session/infra/redis"
)

func (a *application) serviceCmd() *cli.Command {
	return &cli.Command{
		Name: "service",
		Action: func(ctx context.Context, _ *cli.Command) error {
			return service(ctx, a.logger, a.conf)
		},
	}
}

func service(ctx context.Context, logger *slog.Logger, conf config.Env) (err error) {
	svc, closeService, err := newService(ctx, conf)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, closeService())
	}()

	httpServer, err := newHTTPServer(conf)
	if err != nil {
		return err
	}

	grpcServer, err := newGRPCServer(logger, conf, svc)
	if err != nil {
		return err
	}

	errGroup, egCtx := errgroup.WithContext(ctx)
	errGroup.Go(func() error {
		return httpServer.Serve(egCtx)
	})
	errGroup.Go(func() error {
		return grpcServer.Serve(egCtx)
	})
	return errGroup.Wait()
}

func newService(ctx context.Context, conf config.Env) (*app.Service, func() error, error) {
	policy, err := domain.NewPolicy(conf.Session.IdleTTL, conf.Session.AbsoluteTTL)
	if err != nil {
		return nil, nil, err
	}
	client, err := openRedis(ctx, conf.Redis)
	if err != nil {
		return nil, nil, err
	}
	return app.NewService(sessionredis.NewRepositoryProvider(client), policy, time.Now), client.Close, nil
}
