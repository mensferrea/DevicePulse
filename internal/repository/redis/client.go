package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	RDB *redis.Client
}

func NewClient(ctx context.Context, addr, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		PoolSize: 50,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis at %s: %w", addr, err)
	}

	return &Client{RDB: rdb}, nil
}

func (c *Client) Close() error {
	if c.RDB != nil {
		return c.RDB.Close()
	}
	return nil
}
