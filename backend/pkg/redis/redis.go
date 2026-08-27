package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Client wraps redis.Client with additional functionality
type Client struct {
	*redis.Client
	log *logger.Logger
}

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg *config.RedisConfig, log *logger.Logger) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisAddr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Info("Redis connected successfully",
		zap.String("address", cfg.GetRedisAddr()),
		zap.Int("db", cfg.DB),
		zap.Int("pool_size", cfg.PoolSize),
	)

	return &Client{
		Client: client,
		log:    log,
	}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	if err := c.Client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis: %w", err)
	}
	c.log.Info("Redis connection closed")
	return nil
}

// HealthCheck performs a Redis health check
func (c *Client) HealthCheck(ctx context.Context) error {
	if err := c.Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}
	return nil
}

// Helper methods for common operations

// GetJSON gets a JSON value from Redis and unmarshals it
func (c *Client) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := c.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key not found: %s", key)
		}
		return fmt.Errorf("failed to get key: %w", err)
	}

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// SetJSON sets a JSON value in Redis
func (c *Client) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := c.Set(ctx, key, data, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	return nil
}

// DeletePattern deletes all keys matching a pattern
func (c *Client) DeletePattern(ctx context.Context, pattern string) error {
	iter := c.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := c.Del(ctx, iter.Val()).Err(); err != nil {
			c.log.Error("Failed to delete key", zap.String("key", iter.Val()), zap.Error(err))
		}
	}
	return iter.Err()
}

// SetWithRetry sets a value with retry logic
func (c *Client) SetWithRetry(ctx context.Context, key string, value interface{}, expiration time.Duration, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := c.Set(ctx, key, value, expiration).Err()
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
