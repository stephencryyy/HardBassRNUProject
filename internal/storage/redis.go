package storage

import (
	"context"
	"fmt"
	"log"
	"strconv"

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
func (s *RedisClient) SaveSession(sessionID string, sessionData map[string]interface{}) error {
	log.Printf("Saving session %s with data: %v", sessionID, sessionData)
	// Сохранение сессии в Redis...
	err := s.Client.HMSet(ctx, sessionID, sessionData).Err()
	if err != nil {
		log.Printf("Failed to save session %s: %v", sessionID, err)
		return err
	}
	log.Printf("Session %s saved successfully", sessionID)
	return nil
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
	// Используем HIncrBy, чтобы увеличить "uploaded_size" на заданное количество
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
		if key == "uploaded_size" {
			// Преобразуем uploaded_size в целое число
			if uploadedSize, err := strconv.ParseInt(value, 10, 64); err == nil {
				sessionData[key] = uploadedSize
			} else {
				sessionData[key] = 0 // Если ошибка парсинга, ставим 0
			}
		} else {
			sessionData[key] = value
		}
	}

	return sessionData, nil
}

// Метод GetChunks
func (r *RedisClient) GetChunks(sessionID string) ([]int, error) {
	setKey := fmt.Sprintf("%s:chunks", sessionID)
	chunkIDsStr, err := r.Client.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, err
	}
	chunkIDs := []int{}
	for _, idStr := range chunkIDsStr {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		chunkIDs = append(chunkIDs, id)
	}
	return chunkIDs, nil
}

// Получение sessionID по имени файла
func (r *RedisClient) GetSessionIDByFileName(fileName string) (string, error) {
	sessionIDKey := fmt.Sprintf("session:%s", fileName)
	sessionID, err := r.Client.Get(ctx, sessionIDKey).Result()
	if err == redis.Nil {
		return "", nil // Сессия для этого файла не найдена
	}
	if err != nil {
		return "", err // Ошибка при поиске сессии
	}
	return sessionID, nil
}

// Удаление сессии
func (r *RedisClient) DeleteSession(sessionID string) error {
	return r.Client.Del(ctx, sessionID).Err()
}
