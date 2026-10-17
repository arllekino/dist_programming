package main

import (
	"log/slog"

	"github.com/distributed-programming-2026/lib/runtime/http"

	"auth/internal/auth/app"
	"auth/internal/auth/config"
	publichandler "auth/internal/auth/infra/handlers/public"
)

func newHTTPServer(logger *slog.Logger, conf config.Env, svc *app.Service) (*http.Server, error) {
	register, err := publichandler.Register(svc, logger)
	if err != nil {
		return nil, err
	}

	return http.NewServer(http.Config{
		Address:      conf.HTTP.Address,
		GraceTimeout: conf.App.GraceTimeout,
	}, register)
}
