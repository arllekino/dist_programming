package main

import (
	"log/slog"

	"github.com/distributed-programming-2026/go-sdk/pkg/amqp"
	"github.com/distributed-programming-2026/go-sdk/pkg/event"
	eventamqp "github.com/distributed-programming-2026/go-sdk/pkg/event/amqp"

	domainevent "github.com/distributed-programming-2026/lib/event"

	"session/internal/session/app"
	"session/internal/session/config"
	eventhandler "session/internal/session/infra/handlers/events"
)

func newConnection(logger *slog.Logger, conf config.Env) amqp.Connection {
	return amqp.NewConnection(appID, amqp.ConnectionConfig{
		URL:            conf.AMQP.URL,
		ConnectTimeout: conf.AMQP.ConnectTimeout,
	}, logger)
}

func eventQueue(conf config.Env) *amqp.QueueConfig {
	return &amqp.QueueConfig{
		Name:    conf.AMQP.Queue,
		Durable: true,
		DLQ: &amqp.DLQConfig{
			Exchange:   *domainevent.DomainDeadLetterExchange(),
			Queue:      conf.AMQP.Queue + ".dead",
			RoutingKey: conf.AMQP.Queue,
			Durable:    true,
		},
	}
}

func eventBinding(conf config.Env) *amqp.BindConfig {
	return &amqp.BindConfig{
		QueueName:    conf.AMQP.Queue,
		ExchangeName: domainevent.DomainExchange().Name,
		RoutingKeys:  eventhandler.RoutingKeys(),
	}
}

func newMessageHandler(logger *slog.Logger, svc *app.Service) (amqp.Handler, error) {
	return eventamqp.NewHandler(event.ChainHandler(eventhandler.NewHandler(svc, logger), event.HandlerLogging(logger)))
}
