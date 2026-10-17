package main

import (
	"log/slog"

	"github.com/distributed-programming-2026/go-sdk/pkg/amqp"
	"github.com/distributed-programming-2026/go-sdk/pkg/event"
	eventamqp "github.com/distributed-programming-2026/go-sdk/pkg/event/amqp"
	"github.com/distributed-programming-2026/go-sdk/pkg/event/outbox"
	"github.com/distributed-programming-2026/go-sdk/pkg/uow"

	domainevent "github.com/distributed-programming-2026/lib/event"

	"identity/internal/identity/config"
)

func newDispatcher(logger *slog.Logger, conf config.Env, db *database, connection amqp.Connection) (event.Dispatcher, *outbox.Relay, error) {
	producer := connection.Producer(domainevent.DomainExchange(), nil, nil)
	transport, err := eventamqp.NewDispatcher(producer)
	if err != nil {
		return nil, nil, err
	}

	return outbox.Decorate(
		event.ChainDispatcher(transport, event.DispatcherLogging(logger)),
		uow.NewLockableUnitOfWork(db.unit),
		outbox.Config{
			Transport:   "rabbitmq",
			BatchSize:   conf.Relay.BatchSize,
			LockTimeout: conf.Relay.LockTimeout,
		},
	)
}
