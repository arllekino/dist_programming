package main

import (
	"log/slog"

	grpcserver "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/distributed-programming-2026/lib/runtime/grpc"
	"github.com/distributed-programming-2026/lib/runtime/http"

	"session/api/server/interceptorgrpc"
	internalapi "session/api/server/internalapi"
	"session/internal/session/app"
	"session/internal/session/config"
	interceptorhandler "session/internal/session/infra/handlers/interceptor"
	internalhandler "session/internal/session/infra/handlers/internalapi"
)

func newHTTPServer(conf config.Env) (*http.Server, error) {
	return http.NewServer(http.Config{
		Address:      conf.HTTP.Address,
		GraceTimeout: conf.App.GraceTimeout,
	})
}

func newGRPCServer(logger *slog.Logger, conf config.Env, svc *app.Service) (*grpc.Server, error) {
	return grpc.NewServer(grpc.Config{
		Address:      conf.GRPC.Address,
		GraceTimeout: conf.App.GraceTimeout,
	}, func(server *grpcserver.Server) error {
		internalapi.RegisterSessionServiceServer(server, internalhandler.NewHandler(svc, logger))
		interceptorgrpc.RegisterInterceptorServer(server, interceptorhandler.NewHandler(svc, logger))
		reflection.Register(server)
		return nil
	})
}
