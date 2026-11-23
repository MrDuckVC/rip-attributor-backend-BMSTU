// Package redis хранит отозванные токены в Redis.
package redis

import (
	"context"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

// Prefix задаёт префикс ключей отозванных токенов.
const Prefix = "attributor.jwt."

// Client предоставляет доступ к списку отозванных токенов в Redis.
type Client struct{ client *goredis.Client }

// New подключается к Redis и проверяет доступность сервера.
func New(ctx context.Context, address, password string) (*Client, error) {
	client := goredis.NewClient(&goredis.Options{Addr: address, Password: password, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Client{client}, nil
}

// Contains проверяет наличие токена в списке отозванных.
func (c *Client) Contains(ctx context.Context, raw string) (bool, error) {
	count, err := c.client.Exists(ctx, Prefix+raw).Result()
	return count != 0, err
}

// Revoke сохраняет отозванный токен до истечения срока его действия.
func (c *Client) Revoke(ctx context.Context, raw string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}
	return c.client.Set(ctx, Prefix+raw, "1", ttl).Err()
}

// Close закрывает подключение к Redis.
func (c *Client) Close() error { return c.client.Close() }
