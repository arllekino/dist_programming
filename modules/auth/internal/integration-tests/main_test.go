package integration_tests

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/kelseyhightower/envconfig"
	"github.com/testcontainers/testcontainers-go"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"auth/internal/auth/config"
)

const (
	moduleID         = "auth"
	brokerURL        = "amqp://integration:integration-password@rabbitmq:5672/"
	identityPassword = "identity:password@local"
)

var testEnv *environment

type environment struct {
	conf            config.Env
	connector       mysql.Connector
	network         *testcontainers.DockerNetwork
	logs            logBuffer
	closers         []func() error
	sessionAddress  string
	identityAddress string
	gatewayAddress  string
}

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) (exitCode int) {
	testEnv = &environment{}
	defer func() {
		err := testEnv.close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitCode = 1
		}
		if exitCode != 0 {
			fmt.Fprintln(os.Stderr, testEnv.logs.String())
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	err := testEnv.prepare(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return m.Run()
}

func (e *environment) prepare(ctx context.Context) error {
	err := envconfig.Process("integration_defaults", &e.conf)
	if err != nil {
		return err
	}

	e.network, err = network.New(ctx)
	if err != nil {
		return err
	}
	e.closers = append(e.closers, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return e.network.Remove(ctx)
	})

	dsn, err := e.startMySQL(ctx)
	if err != nil {
		return err
	}
	err = e.startRedis(ctx)
	if err != nil {
		return err
	}
	err = e.startBroker(ctx)
	if err != nil {
		return err
	}

	e.connector = mysql.NewConnector()
	err = e.connector.Open(dsn, mysql.Config{
		MaxConnections: 10,
	})
	if err != nil {
		return err
	}
	e.closers = append(e.closers, e.connector.Close)

	e.conf.App.GraceTimeout = time.Second
	e.conf.Relay.Interval = 50 * time.Millisecond
	e.conf.Relay.PublishTimeout = time.Second
	return e.startApplications(ctx)
}

func (e *environment) startMySQL(ctx context.Context) (string, error) {
	script := filepath.Join(os.TempDir(), fmt.Sprintf("auth-integration-identity-%d.sql", os.Getpid()))
	err := os.WriteFile(script, []byte(`
CREATE DATABASE IF NOT EXISTS identity CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'identity'@'%' IDENTIFIED BY '`+identityPassword+`';
GRANT ALL PRIVILEGES ON identity.* TO 'identity'@'%';
`), 0o600)
	if err != nil {
		return "", err
	}
	e.closers = append(e.closers, func() error {
		return os.Remove(script)
	})

	mysqlContainer, err := tcmysql.Run(ctx, "mysql:8.4",
		tcmysql.WithDatabase(moduleID+"_test"),
		tcmysql.WithUsername(moduleID),
		tcmysql.WithPassword("integration:password@local"),
		tcmysql.WithScripts(script),
		network.WithNetwork([]string{"mysql"}, e.network),
	)
	if mysqlContainer != nil {
		e.closers = append(e.closers, func() error {
			return testcontainers.TerminateContainer(mysqlContainer)
		})
	}
	if err != nil {
		return "", err
	}

	dsn, err := mysqlContainer.ConnectionString(ctx, "parseTime=true")
	if err != nil {
		return "", err
	}
	databaseConfig, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return "", err
	}
	e.conf.MySQL.Address = databaseConfig.Addr
	e.conf.MySQL.Database = databaseConfig.DBName
	e.conf.MySQL.User = databaseConfig.User
	e.conf.MySQL.Password = databaseConfig.Passwd
	return dsn, nil
}

func (e *environment) startRedis(ctx context.Context) error {
	redisContainer, err := tcredis.Run(ctx, "redis:7.4-alpine",
		network.WithNetwork([]string{"redis"}, e.network),
	)
	if redisContainer != nil {
		e.closers = append(e.closers, func() error {
			return testcontainers.TerminateContainer(redisContainer)
		})
	}
	return err
}

func (e *environment) startBroker(ctx context.Context) error {
	broker, err := e.startContainer(ctx, testcontainers.ContainerRequest{
		Image:        "rabbitmq:4.1-alpine",
		ExposedPorts: []string{"5672/tcp"},
		Networks:     []string{e.network.Name},
		NetworkAliases: map[string][]string{
			e.network.Name: {"rabbitmq"},
		},
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "integration",
			"RABBITMQ_DEFAULT_PASS": "integration-password",
		},
		WaitingFor: wait.ForLog("Server startup complete").WithStartupTimeout(time.Minute),
	})
	if err != nil {
		return err
	}
	address, err := containerAddress(ctx, broker, "5672/tcp")
	if err != nil {
		return err
	}
	e.conf.AMQP.URL = (&url.URL{
		Scheme: "amqp",
		User:   url.UserPassword("integration", "integration-password"),
		Host:   address,
		Path:   "/",
	}).String()
	return nil
}

func (e *environment) startApplications(ctx context.Context) error {
	sessionEnv := map[string]string{
		"SESSION_REDIS_ADDRESS":     "redis:6379",
		"SESSION_AMQP_URL":          brokerURL,
		"SESSION_APP_GRACE_TIMEOUT": e.conf.App.GraceTimeout.String(),
	}
	session, err := e.startApplication(ctx, "session", "service", "session-service", sessionEnv)
	if err != nil {
		return err
	}
	_, err = e.startApplication(ctx, "session", "message-handler", "", sessionEnv)
	if err != nil {
		return err
	}
	e.sessionAddress, err = containerAddress(ctx, session, "8081/tcp")
	if err != nil {
		return err
	}

	identity, err := e.startApplication(ctx, "identity", "service", "identity-service", map[string]string{
		"IDENTITY_MYSQL_ADDRESS":     "mysql:3306",
		"IDENTITY_MYSQL_DATABASE":    "identity",
		"IDENTITY_MYSQL_USER":        "identity",
		"IDENTITY_MYSQL_PASSWORD":    identityPassword,
		"IDENTITY_AMQP_URL":          brokerURL,
		"IDENTITY_APP_GRACE_TIMEOUT": e.conf.App.GraceTimeout.String(),
	})
	if err != nil {
		return err
	}
	e.identityAddress, err = containerAddress(ctx, identity, "8081/tcp")
	if err != nil {
		return err
	}

	auth, err := e.startApplication(ctx, moduleID, "service", "auth-service", map[string]string{
		"AUTH_MYSQL_ADDRESS":         "mysql:3306",
		"AUTH_MYSQL_DATABASE":        e.conf.MySQL.Database,
		"AUTH_MYSQL_USER":            e.conf.MySQL.User,
		"AUTH_MYSQL_PASSWORD":        e.conf.MySQL.Password,
		"AUTH_AMQP_URL":              brokerURL,
		"AUTH_SESSION_ADDRESS":       "session-service:8081",
		"AUTH_IDENTITY_ADDRESS":      "identity-service:8081",
		"AUTH_PASSWORD_HASH_COST":    "4",
		"AUTH_HTTP_ADDRESS":          ":8082",
		"AUTH_APP_GRACE_TIMEOUT":     e.conf.App.GraceTimeout.String(),
		"AUTH_RELAY_INTERVAL":        e.conf.Relay.Interval.String(),
		"AUTH_RELAY_PUBLISH_TIMEOUT": e.conf.Relay.PublishTimeout.String(),
	})
	if err != nil {
		return err
	}
	e.conf.HTTP.Address, err = containerAddress(ctx, auth, "8082/tcp")
	if err != nil {
		return err
	}

	gatewayConfig, err := filepath.Glob(filepath.Join("..", "..", "..", "..", "deploy", "gateway", "config", "*.yaml"))
	if err != nil {
		return err
	}
	if len(gatewayConfig) == 0 {
		return errors.New("gateway configuration not found in deploy/gateway/config")
	}
	files := make([]testcontainers.ContainerFile, 0, len(gatewayConfig))
	for _, path := range gatewayConfig {
		files = append(files, testcontainers.ContainerFile{
			HostFilePath:      path,
			ContainerFilePath: "/config/" + filepath.Base(path),
			FileMode:          0o644,
		})
	}

	gateway, err := e.startContainer(ctx, testcontainers.ContainerRequest{
		Image:        "distributed-programming-gateway:integration",
		ExposedPorts: []string{"8000/tcp", "15000/tcp"},
		Networks:     []string{e.network.Name},
		Files:        files,
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&e.logs},
		},
		WaitingFor: wait.ForHTTP("/readyz").WithPort("15000/tcp").WithStatusCodeMatcher(func(status int) bool {
			return status == http.StatusNoContent
		}).WithStartupTimeout(time.Minute),
	})
	if err != nil {
		return err
	}
	e.gatewayAddress, err = containerAddress(ctx, gateway, "8000/tcp")
	return err
}

func (e *environment) startApplication(ctx context.Context, module, command, alias string, env map[string]string) (testcontainers.Container, error) {
	request := testcontainers.ContainerRequest{
		Image:        "distributed-programming-" + module + ":integration",
		Cmd:          []string{command},
		ExposedPorts: []string{"8081/tcp", "8082/tcp"},
		Networks:     []string{e.network.Name},
		Env:          maps.Clone(env),
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&e.logs},
		},
		WaitingFor: wait.ForHTTP("/healthz").WithPort("8082/tcp").WithStatusCodeMatcher(func(status int) bool {
			return status == http.StatusNoContent
		}).WithStartupTimeout(time.Minute),
	}
	if alias != "" {
		request.NetworkAliases = map[string][]string{e.network.Name: {alias}}
	}
	return e.startContainer(ctx, request)
}

func (e *environment) startContainer(ctx context.Context, request testcontainers.ContainerRequest) (testcontainers.Container, error) {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: request,
		Started:          true,
	})
	if container != nil {
		e.closers = append(e.closers, func() error {
			return testcontainers.TerminateContainer(container)
		})
	}
	return container, err
}

func (e *environment) close() error {
	var errs []error
	for i := len(e.closers) - 1; i >= 0; i-- {
		errs = append(errs, e.closers[i]())
	}
	return errors.Join(errs...)
}

func containerAddress(ctx context.Context, container testcontainers.Container, port string) (string, error) {
	host, err := container.Host(ctx)
	if err != nil {
		return "", err
	}
	mappedPort, err := container.MappedPort(ctx, port)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(host, mappedPort.Port()), nil
}
