package main

import (
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"auth/internal/auth/app"
	"auth/internal/auth/config"
	"auth/internal/auth/infra/clients"
)

type dependencies struct {
	sessions    app.SessionClient
	identity    app.IdentityClient
	connections []*grpc.ClientConn
}

func newDependencies(conf config.Env) (_ *dependencies, err error) {
	deps := &dependencies{}
	defer func() {
		if err != nil {
			err = errors.Join(err, deps.close())
		}
	}()

	sessionConnection, err := deps.createConnection(conf.Session.Address)
	if err != nil {
		return nil, err
	}
	identityConnection, err := deps.createConnection(conf.Identity.Address)
	if err != nil {
		return nil, err
	}
	deps.sessions = clients.NewSessionClient(sessionConnection, conf.Session.Timeout)
	deps.identity = clients.NewIdentityClient(identityConnection, conf.Identity.Timeout)
	return deps, nil
}

func (d *dependencies) createConnection(address string) (*grpc.ClientConn, error) {
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	d.connections = append(d.connections, connection)
	return connection, nil
}

func (d *dependencies) close() error {
	errs := make([]error, 0, len(d.connections))
	for _, connection := range d.connections {
		errs = append(errs, connection.Close())
	}
	return errors.Join(errs...)
}
