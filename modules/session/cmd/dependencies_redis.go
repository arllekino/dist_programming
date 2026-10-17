package main

import (
	"context"
	"errors"

	goredis "github.com/redis/go-redis/v9"

	"session/internal/session/config"
)

func openRedis(ctx context.Context, conf config.Redis) (_ *goredis.Client, err error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:         conf.Address,
		Password:     conf.Password,
		DB:           conf.DB,
		DialTimeout:  conf.DialTimeout,
		ReadTimeout:  conf.ReadTimeout,
		WriteTimeout: conf.WriteTimeout,
		PoolSize:     conf.PoolSize,
	})
	defer func() {
		if err != nil {
			err = errors.Join(err, client.Close())
		}
	}()

	err = client.Ping(ctx).Err()
	if err != nil {
		return nil, err
	}
	return client, nil
}
