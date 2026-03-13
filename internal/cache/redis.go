package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RedisCache provides Redis-backed caching
type RedisCache struct {
	client *redis.Client
	ctx    context.Context
	logger *zap.Logger
	prefix string
}

// NewRedisCache creates a new Redis cache client
func NewRedisCache(addr, password string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{
		client: client,
		ctx:    ctx,
		prefix: "nas-os:",
	}, nil
}

// SetWithLogger sets logger for redis cache
func (r *RedisCache) SetWithLogger(logger *zap.Logger) {
	r.logger = logger
}

// Get retrieves a value from Redis
func (r *RedisCache) Get(key string) (interface{}, bool) {
	val, err := r.client.Get(r.ctx, r.prefix+key).Bytes()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		if r.logger != nil {
			r.logger.Error("Redis get error", zap.Error(err), zap.String("key", key))
		}
		return nil, false
	}

	// Try to unmarshal as JSON first
	var result interface{}
	if err := json.Unmarshal(val, &result); err != nil {
		// If not JSON, return as string
		return string(val), true
	}

	return result, true
}

// Set stores a value in Redis with default TTL
func (r *RedisCache) Set(key string, value interface{}) error {
	return r.SetWithTTL(key, value, 1*time.Hour)
}

// SetWithTTL stores a value with custom TTL
func (r *RedisCache) SetWithTTL(key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.client.Set(r.ctx, r.prefix+key, data, ttl).Err()
}

// Delete removes a key from Redis
func (r *RedisCache) Delete(key string) error {
	return r.client.Del(r.ctx, r.prefix+key).Err()
}

// Exists checks if a key exists
func (r *RedisCache) Exists(key string) (bool, error) {
	val, err := r.client.Exists(r.ctx, r.prefix+key).Result()
	return val > 0, err
}

// Clear clears all keys with our prefix
func (r *RedisCache) Clear() error {
	keys, err := r.client.Keys(r.ctx, r.prefix+"*").Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return r.client.Del(r.ctx, keys...).Err()
	}
	return nil
}

// GetClient returns the underlying redis client
func (r *RedisCache) GetClient() *redis.Client {
	return r.client
}

// Close closes the redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}
