package storage

import (
	"context"
	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

type RedisClient struct {
	Client *redis.Client
}

func NewRedisClient(addr, password string, db int) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisClient{
		Client: rdb,
	}
}

func (r *RedisClient) SaveSession(sessionID string, data map[string]interface{}) error {
	return r.Client.HSet(ctx, sessionID, data).Err()
}

func (r *RedisClient) SessionExists(sessionID string) (int64, error) {
	return r.Client.Exists(ctx, sessionID).Result()
}
