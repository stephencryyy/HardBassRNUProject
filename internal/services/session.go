package services

import (
	"BASProject/internal/storage"
	"BASProject/internal/utils"
	"errors"
	"fmt"
)

type SessionService struct {
	Storage         *storage.RedisClient
	ChecksumService *utils.ChecksumService
}

func NewSessionService(storage *storage.RedisClient) *SessionService {
	return &SessionService{
		Storage:         storage,
		ChecksumService: utils.NewChecksumService(),
	}
}

// Создание сессии загрузки файла
func (s *SessionService) CreateSession(fileName string, fileSize int64) (string, int64, error) {
	if fileName == "" || fileSize <= 0 {
		return "", 0, errors.New("invalid file name or file size")
	}

	sessionID := utils.GenerateSessionID()
	chunkSize := int64(4 * 1024 * 1024) // 4MB

	sessionData := map[string]interface{}{
		"file_name":     fileName,
		"file_size":     fileSize,
		"chunk_size":    chunkSize,
		"uploaded_size": 0,
	}

	err := s.Storage.SaveSession(sessionID, sessionData)
	if err != nil {
		return "", 0, err
	}

	return sessionID, chunkSize, nil
}

// Сохранение чанка и обновление прогресса загрузки
func (s *SessionService) SaveChunk(sessionID string, chunkID int, chunkData []byte) error {
	// Проверка, загружен ли чанк
	chunkExists, err := s.Storage.ChunkExists(sessionID, chunkID)
	if err != nil {
		return fmt.Errorf("failed to check if chunk exists: %w", err)
	}
	if chunkExists {
		return errors.New("chunk already exists")
	}

	// Сохранение чанка в Redis
	err = s.Storage.SaveChunkData(sessionID, chunkID, chunkData)
	if err != nil {
		return fmt.Errorf("failed to save chunk data: %w", err)
	}

	// Обновление общего прогресса загрузки
	err = s.Storage.UpdateUploadedSize(sessionID, int64(len(chunkData)))
	if err != nil {
		return fmt.Errorf("failed to update uploaded size: %w", err)
	}

	return nil
}

// Проверка состояния загрузки сессии
func (s *SessionService) GetUploadStatus(sessionID string) (map[string]interface{}, error) {
	// Получаем данные сессии из Redis
	sessionData, err := s.Storage.GetSessionData(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve session data: %w", err)
	}

	// Проверяем, завершена ли загрузка
	fileSize, _ := sessionData["file_size"].(int64)
	uploadedSize, _ := sessionData["uploaded_size"].(int64)
	isComplete := uploadedSize >= fileSize

	status := map[string]interface{}{
		"file_size":      fileSize,
		"uploaded_size":  uploadedSize,
		"completed":      isComplete,
		"remaining_size": fileSize - uploadedSize,
	}

	return status, nil
}

// Метод GetNextChunkID для получения следующего доступного ID чанка
func (s *SessionService) GetNextChunkID(sessionID string) (int, error) {
	// Получаем список всех чанков для данной сессии из Redis
	chunks, err := s.Storage.GetChunks(sessionID)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve chunks: %w", err)
	}

	// Ищем максимальный существующий ID чанка
	maxChunkID := -1
	for _, chunk := range chunks {
		if chunkID, ok := chunk["chunk_id"].(int); ok {
			if chunkID > maxChunkID {
				maxChunkID = chunkID
			}
		}
	}

	// Следующий доступный ID будет на 1 больше максимального
	nextChunkID := maxChunkID + 1
	return nextChunkID, nil
}
