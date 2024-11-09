package services

import (
	"BASProject/internal/storage"
	"BASProject/internal/utils"
	"errors"
	"fmt"
)

type SessionService struct {
	Storage     *storage.RedisClient
	FileService *FileService
}

func NewSessionService(storage *storage.RedisClient, fileService *FileService) *SessionService {
	return &SessionService{
		Storage:     storage,
		FileService: fileService,
	}
}

// Создание сессии загрузки файла с проверкой на существование файла
func (s *SessionService) CreateSession(fileName string, fileSize int64) (string, int64, error) {
	if fileName == "" || fileSize <= 0 {
		return "", 0, errors.New("invalid file name or file size")
	}

	// Проверка наличия файла на сервере
	if !s.FileService.FileExists(fileName) {
		return "", 0, errors.New("file not found on the server")
	}

	sessionID := utils.GenerateSessionID()
	chunkSize := s.FileService.CalculateChunkSize(fileSize)

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

// Получение состояния загрузки для сессии
func (s *SessionService) GetUploadStatus(sessionID string) (map[string]interface{}, error) {
	// Получаем данные сессии из Redis
	sessionData, err := s.Storage.GetSessionData(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve session data: %w", err)
	}

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
