package services

import (
	"BASProject/internal/storage"
	"BASProject/internal/utils"
	"errors"
)

type SessionService struct {
	Storage *storage.RedisClient
}

func NewSessionService(storage *storage.RedisClient) *SessionService {
	return &SessionService{
		Storage: storage,
	}
}

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
