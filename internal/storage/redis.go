package storage

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

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

    // Convert all values to strings
    dataToSave := make(map[string]interface{})
    for key, value := range sessionData {
        dataToSave[key] = fmt.Sprintf("%v", value)
    }

    err := s.Client.HMSet(ctx, sessionID, dataToSave).Err()
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

// Добавление chunkID в множество загруженных чанков
func (r *RedisClient) AddUploadedChunk(sessionID string, chunkID int) error {
	setKey := fmt.Sprintf("%s:chunks", sessionID)
	return r.Client.SAdd(ctx, setKey, chunkID).Err()
}

// Проверка, загружен ли чанк
func (r *RedisClient) ChunkExists(sessionID string, chunkID int) (bool, error) {
	chunkKey := fmt.Sprintf("%s:chunks", sessionID)
	exists, err := r.Client.SIsMember(ctx, chunkKey, chunkID).Result()
	return exists, err
}

// Обновление загруженного объема данных в сессии
func (r *RedisClient) UpdateUploadedSize(sessionID string, size int64) error {
	// Используем HIncrBy, чтобы увеличить "uploaded_size" на заданное количество
	return r.Client.HIncrBy(ctx, sessionID, "uploaded_size", size).Err()
}

// Получение данных сессии
func (r *RedisClient) GetSessionData(sessionID string) (map[string]interface{}, error) {
	// Проверяем, существует ли ключ с данным sessionID
	exists, err := r.Client.Exists(ctx, sessionID).Result()
	if err != nil {
		return nil, fmt.Errorf("error checking existence of session: %v", err)
	}
	if exists == 0 {
		// Если ключ не существует, возвращаем ошибку
		return nil, fmt.Errorf("session not found")
	}

	// Извлекаем данные сессии из Redis
	data, err := r.Client.HGetAll(ctx, sessionID).Result()
	if err != nil {
		return nil, err
	}

	// Создаем карту для хранения данных сессии
	sessionData := make(map[string]interface{})
	for key, value := range data {
		switch key {
		case "file_size", "uploaded_size", "chunk_size":
			// Преобразуем значения в int64
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

// Удаление сессии
func (r *RedisClient) DeleteSessionData(sessionID string) error {
	// Удаляем хэш сессии
	err := r.Client.Del(ctx, sessionID).Err()
	if err != nil {
		return fmt.Errorf("failed to delete session hash: %w", err)
	}

	// Удаляем множество загруженных чанков (set)
	chunksSetKey := fmt.Sprintf("%s:chunks", sessionID)
	err = r.Client.Del(ctx, chunksSetKey).Err()
	if err != nil {
		return fmt.Errorf("failed to delete chunks set: %w", err)
	}

	// Дополнительно проверим, что ключ удален
	exists, err := r.Client.Exists(ctx, chunksSetKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check if chunks set exists: %w", err)
	}

	if exists == 1 {
		log.Printf("Chunks set %s still exists", chunksSetKey)
	} else {
		log.Printf("Chunks set %s successfully deleted", chunksSetKey)
	}

	return nil
}

func (r *RedisClient) AcquireLock(key string, ttl int) (bool, error) {
	result, err := r.Client.SetNX(ctx, key, "locked", time.Duration(ttl)*time.Second).Result()
	return result, err
}

func (r *RedisClient) ReleaseLock(key string) error {
	_, err := r.Client.Del(ctx, key).Result()
	return err
}
