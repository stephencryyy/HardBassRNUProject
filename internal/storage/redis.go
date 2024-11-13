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

// Получение числового значения поля сессии
func (r *RedisClient) GetSessionIntField(sessionID string, field string) (int64, error) {
	val, err := r.Client.HGet(ctx, sessionID, field).Result()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

// Проверка, существует ли сессия
func (r *RedisClient) SessionExists(sessionID string) (int64, error) {
	return r.Client.Exists(ctx, sessionID).Result()
}

// Сохранение данных чанка
func (r *RedisClient) SaveChunkData(sessionID string, chunkID int, chunkData []byte) error {
	chunkKey := fmt.Sprintf("%s:chunk:%d", sessionID, chunkID)
	// Добавляем chunkID в множество загруженных чанков
	err := r.AddUploadedChunk(sessionID, chunkID)
	if err != nil {
		return err
	}
	return r.Client.Set(ctx, chunkKey, chunkData, 0).Err()
}

// Добавление chunkID в множество загруженных чанков
func (r *RedisClient) AddUploadedChunk(sessionID string, chunkID int) error {
	setKey := fmt.Sprintf("%s:chunks", sessionID)
	return r.Client.SAdd(ctx, setKey, chunkID).Err()
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
		switch key {
		case "file_size", "uploaded_size", "chunk_size":
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid value for %s: %v", key, err)
			}
			sessionData[key] = intVal
		default:
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
func (r *RedisClient) DeleteSessionData(sessionID string) error {
	// Удаляем хэш сессии
	err := r.Client.Del(ctx, sessionID).Err()
	if err != nil {
		return err
	}

	// Удаляем множество загруженных чанков
	chunksSetKey := fmt.Sprintf("%s:chunks", sessionID)
	err = r.Client.Del(ctx, chunksSetKey).Err()
	if err != nil {
		return err
	}

	// Удаляем ключи чанков (если вы сохраняете чанки в Redis)
	chunkKeysPattern := fmt.Sprintf("%s:chunk:*", sessionID)
	keys, err := r.Client.Keys(ctx, chunkKeysPattern).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		err = r.Client.Del(ctx, keys...).Err()
		if err != nil {
			return err
		}
	}

	return nil
}
