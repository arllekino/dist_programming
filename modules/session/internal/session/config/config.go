package config

import (
	"errors"
	"time"
)

type Env struct {
	App     App     `envconfig:"app"`
	GRPC    GRPC    `envconfig:"grpc"`
	HTTP    HTTP    `envconfig:"http"`
	Redis   Redis   `envconfig:"redis"`
	AMQP    AMQP    `envconfig:"amqp"`
	Session Session `envconfig:"session"`
}

type App struct {
	Debug        bool          `envconfig:"debug"`
	GraceTimeout time.Duration `envconfig:"grace_timeout" default:"30s"`
}

type GRPC struct {
	Address string `envconfig:"address" default:":8081"`
}

type HTTP struct {
	Address string `envconfig:"address" default:":8082"`
}

type Redis struct {
	Address      string        `envconfig:"address" default:"localhost:6379"`
	Password     string        `envconfig:"password"`
	DB           int           `envconfig:"db" default:"0"`
	DialTimeout  time.Duration `envconfig:"dial_timeout" default:"5s"`
	ReadTimeout  time.Duration `envconfig:"read_timeout" default:"3s"`
	WriteTimeout time.Duration `envconfig:"write_timeout" default:"3s"`
	PoolSize     int           `envconfig:"pool_size" default:"20"`
}

type AMQP struct {
	URL            string        `envconfig:"url" default:"amqp://local:local@localhost:5672/"`
	Queue          string        `envconfig:"queue" default:"session.auth-events"`
	ConnectTimeout time.Duration `envconfig:"connect_timeout" default:"5s"`
	PrefetchCount  int           `envconfig:"prefetch_count" default:"10"`
}

type Session struct {
	IdleTTL     time.Duration `envconfig:"idle_ttl" default:"24h"`
	AbsoluteTTL time.Duration `envconfig:"absolute_ttl" default:"720h"`
}

func (c Env) Validate() error {
	if c.App.GraceTimeout <= 0 || c.AMQP.ConnectTimeout <= 0 {
		return errors.New("timeouts must be positive")
	}
	if c.Redis.DialTimeout <= 0 || c.Redis.ReadTimeout <= 0 || c.Redis.WriteTimeout <= 0 || c.Redis.PoolSize <= 0 || c.Redis.DB < 0 {
		return errors.New("redis timeouts and pool size must be positive; DB must not be negative")
	}
	if c.Session.IdleTTL <= 0 || c.Session.AbsoluteTTL < c.Session.IdleTTL {
		return errors.New("session idle TTL must be positive and not exceed the absolute TTL")
	}
	if c.AMQP.PrefetchCount <= 0 {
		return errors.New("prefetch count must be positive")
	}
	if c.Redis.Address == "" || c.AMQP.URL == "" || c.AMQP.Queue == "" || c.HTTP.Address == "" || c.GRPC.Address == "" {
		return errors.New("storage, broker and server configuration must not be empty")
	}
	return nil
}
