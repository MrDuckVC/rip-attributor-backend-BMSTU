package redis

import (
	"context"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

const Prefix = "attributor.jwt."

type Client struct{ client *goredis.Client }

func New(ctx context.Context, address, password string) (*Client, error) {
	client := goredis.NewClient(&goredis.Options{Addr: address, Password: password, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1})
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}
	return &Client{client}, nil
}

func (c *Client) Contains(ctx context.Context, raw string) (bool, error) {
	count, err := c.client.Exists(ctx, Prefix+raw).Result()
	return count != 0, err
}

func (c *Client) Revoke(ctx context.Context, raw string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}
	return c.client.Set(ctx, Prefix+raw, "1", ttl).Err()
}

func (c *Client) Close() error { return c.client.Close() }
