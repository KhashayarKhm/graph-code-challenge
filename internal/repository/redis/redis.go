package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

type DB struct {
	client *goredis.Client
}

func New(ctx context.Context, url string) (*DB, error) {
	options, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}

	client := goredis.NewClient(options)

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()

		return nil, fmt.Errorf("redis: connect: %w", err)
	}

	return &DB{client: client}, nil
}

func (d *DB) Client() *goredis.Client {
	return d.client
}

func (d *DB) Close() error {
	if err := d.client.Close(); err != nil {
		return fmt.Errorf("redis: close: %w", err)
	}

	return nil
}
