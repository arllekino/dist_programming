package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"

	"auth/internal/auth/app"
	"auth/internal/auth/config"
	eventinfra "auth/internal/auth/infra/event"
	"auth/internal/auth/infra/password"
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
	db, err := openDatabase(ctx, logger, conf)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, db.connector.Close())
	}()

	deps, err := newDependencies(conf)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, deps.close())
	}()

	connection := newConnection(logger, conf)
	defer func() {
		err = errors.Join(err, connection.Stop())
	}()

	dispatcher, relay, err := newDispatcher(logger, conf, db, connection)
	if err != nil {
		return err
	}

	err = connection.Start()
	if err != nil {
		return err
	}

	dispatcherFactory := eventinfra.NewDispatcherFactory(appID, dispatcher)
	svc, err := app.NewService(
		db.unit,
		dispatcherFactory,
		password.NewBcryptHasher(conf.Password.HashCost),
		deps.sessions,
		deps.identity,
		logger,
		time.Now,
	)
	if err != nil {
		return err
	}

	httpServer, err := newHTTPServer(logger, conf, svc)
	if err != nil {
		return err
	}

	errGroup, egCtx := errgroup.WithContext(ctx)
	errGroup.Go(func() error {
		return httpServer.Serve(egCtx)
	})
	errGroup.Go(func() error {
		return runRelay(egCtx, logger, relay, conf.Relay)
	})
	return errGroup.Wait()
}
