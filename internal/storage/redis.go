package storage

import (
	"context"
	"fmt"
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

// Сохранение сессии
func (r *RedisClient) SaveSession(sessionID string, data map[string]interface{}) error {
	return r.Client.HSet(ctx, sessionID, data).Err()
}

// Проверка, существует ли сессия
func (r *RedisClient) SessionExists(sessionID string) (int64, error) {
	return r.Client.Exists(ctx, sessionID).Result()
}

// Сохранение данных чанка
func (r *RedisClient) SaveChunkData(sessionID string, chunkID int, chunkData []byte) error {
	chunkKey := fmt.Sprintf("%s:chunk:%d", sessionID, chunkID)
	return r.Client.Set(ctx, chunkKey, chunkData, 0).Err()
}

// Проверка, загружен ли чанк
func (r *RedisClient) ChunkExists(sessionID string, chunkID int) (bool, error) {
	chunkKey := fmt.Sprintf("%s:chunk:%d", sessionID, chunkID)
	exists, err := r.Client.Exists(ctx, chunkKey).Result()
	return exists > 0, err
}

// Обновление загруженного объема данных в сессии
func (r *RedisClient) UpdateUploadedSize(sessionID string, size int64) error {
	return r.Client.HIncrBy(ctx, sessionID, "uploaded_size", size).Err()
}

// Получение данных сессии
func (r *RedisClient) GetSessionData(sessionID string) (map[string]interface{}, error) {
	data, err := r.Client.HGetAll(ctx, sessionID).Result()
	if err != nil {
		return nil, err
	}

	sessionData := make(map[string]interface{})
	for key, value := range data {
		sessionData[key] = value
	}

	return sessionData, nil
}
