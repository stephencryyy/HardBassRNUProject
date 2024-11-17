package services

import (
	"BASProject/internal/storage"
	"errors"
	"fmt"
	"log"
)

type SessionService struct {
	Storage     *storage.RedisClient
	FileService *FileService
}

type ISessionService interface {
	CreateSession(fileName string, fileSize int64, fileHash string) (int64, error)
	GetUploadStatus(fileHash string) (map[string]interface{}, error)
	UpdateProgress(fileHash string, uploadedSize int64) error
	DeleteSession(fileHash string) error
	GetFileService() IFileService
}

func NewSessionService(storage *storage.RedisClient, fileService *FileService) *SessionService {
	return &SessionService{
		Storage:     storage,
		FileService: fileService,
	}
}

var ErrSessionNotFound = errors.New("session not found")

// CreateSession creates a new file upload session using the file hash provided by the client.
func (s *SessionService) CreateSession(fileName string, fileSize int64, fileHash string) (int64, error) {
	if fileName == "" || fileSize <= 0 || fileHash == "" {
		return 0, errors.New("invalid file name, file size, or file hash")
	}

	// Проверяем, существует ли сессия по хешу
	exists, err := s.Storage.SessionExists(fileHash)
	if err != nil {
		return 0, fmt.Errorf("failed to check session existence: %w", err)
	}
	if exists > 0 {
		// Если сессия существует, получаем её статус
		sessionData, err := s.Storage.GetSessionData(fileHash)
		if err != nil {
			return 0, fmt.Errorf("failed to retrieve session data: %w", err)
		}

		sessionStatus, ok := sessionData["status"].(string)
		if !ok {
			log.Printf("Invalid session status: %v", sessionData)
			return 0, errors.New("invalid session status")
		}

		switch sessionStatus {
		case "completed":
			// Удаляем текущую сессию и начинаем заново
			err = s.DeleteSession(fileHash)
			if err != nil {
				return 0, fmt.Errorf("failed to delete completed session: %w", err)
			}

		case "in_progress":
			// Возвращаем существующую информацию о чанках
			return sessionData["chunk_size"].(int64), nil
		}
	}

	// Новая сессия: определяем размер чанков
	maxChunkSize := int64(1024 * 1024 * 1024) // 1GB max chunk size
	chunkSize := s.FileService.CalculateChunkSize(fileSize, maxChunkSize)

	log.Printf("Creating session with fileHash: %s", fileHash)
	sessionData := map[string]interface{}{
		"file_name":     fileName,
		"file_size":     fileSize,
		"chunk_size":    chunkSize,
		"uploaded_size": 0,
		"status":        "in_progress",
	}
	err = s.Storage.SaveSession(fileHash, sessionData)
	if err != nil {
		log.Printf("Error saving session to Redis: %v", err)
		return 0, fmt.Errorf("failed to save session: %w", err)
	}
	log.Printf("Session %s saved successfully", fileHash)
	return chunkSize, nil
}

// UpdateProgress updates the progress of the file upload for the specified file hash.
func (s *SessionService) UpdateProgress(fileHash string, uploadedSize int64) error {
	sessionData, err := s.Storage.GetSessionData(fileHash)
	if err != nil {
		log.Printf("Error retrieving session data: %v", err)
		return fmt.Errorf("failed to retrieve session data: %w", err)
	}

	sessionData["uploaded_size"] = uploadedSize
	err = s.Storage.SaveSession(fileHash, sessionData)
	if err != nil {
		log.Printf("Error saving updated session data: %v", err)
		return fmt.Errorf("failed to save updated session data: %w", err)
	}

	// Проверка завершенности загрузки
	fileSize := sessionData["file_size"].(int64)
	if uploadedSize >= fileSize {
		sessionData["status"] = "completed"
		err = s.Storage.SaveSession(fileHash, sessionData)
		if err != nil {
			log.Printf("Error marking session as complete: %v", err)
			return fmt.Errorf("failed to mark session as completed: %w", err)
		}
		log.Printf("Session %s marked as completed", fileHash)
	}

	return nil
}

// GetUploadStatus retrieves the current status of the upload for the specified file hash.
func (s *SessionService) GetUploadStatus(fileHash string) (map[string]interface{}, error) {
	// Получаем данные сессии из Redis
	exists, err := s.Storage.SessionExists(fileHash)
	if err != nil {
		return nil, fmt.Errorf("failed to check session existence: %w", err)
	}
	if exists == 0 {
		return nil, ErrSessionNotFound
	}

	sessionData, err := s.Storage.GetSessionData(fileHash)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve session data: %w", err)
	}

	fileSize, ok := sessionData["file_size"].(int64)
	if !ok {
		return nil, fmt.Errorf("invalid file size in session data")
	}
	uploadedSize, ok := sessionData["uploaded_size"].(int64)
	if !ok {
		return nil, fmt.Errorf("invalid uploaded size in session data")
	}
	chunkSize, ok := sessionData["chunk_size"].(int64)
	if !ok {
		return nil, fmt.Errorf("invalid chunk size in session data")
	}

	// Получаем список загруженных чанков
	uploadedChunks, err := s.Storage.GetChunks(fileHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get uploaded chunks: %w", err)
	}

	// Определяем список ожидающих чанков
	pendingChunks := []int{}
	uploadedChunksMap := make(map[int]bool)
	for _, chunkID := range uploadedChunks {
		uploadedChunksMap[chunkID] = true
	}

	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)

	for i := 1; i <= totalChunks; i++ {
		if !uploadedChunksMap[i] {
			pendingChunks = append(pendingChunks, i)
		}
	}

	isComplete := len(pendingChunks) == 0
	message := "Upload is in progress."
	if isComplete {
		message = "Upload is complete."
	}

	// Формируем статус загрузки
	status := map[string]interface{}{
		"file_size":       fileSize,
		"uploaded_size":   uploadedSize,
		"completed":       isComplete,
		"remaining_size":  fileSize - uploadedSize,
		"status":          sessionData["status"].(string),
		"uploaded_chunks": uploadedChunks,
		"pending_chunks":  pendingChunks,
		"total_chunks":    totalChunks,
		"message":         message,
	}

	return status, nil
}

// DeleteSession deletes a session and its associated chunk files.
func (s *SessionService) DeleteSession(fileHash string) error {
	// Проверяем, существует ли сессия
	exists, err := s.Storage.SessionExists(fileHash)
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}
	if exists == 0 {
		return ErrSessionNotFound
	}
	err = s.Storage.DeleteSessionData(fileHash)
	if err != nil {
		return fmt.Errorf("failed to delete session data: %w", err)
	}

	// Удаляем файлы чанков с диска
	err = s.FileService.DeleteChunks(fileHash)
	if err != nil {
		return fmt.Errorf("failed to delete chunk files: %w", err)
	}

	return nil
}

func (s *SessionService) GetFileService() IFileService {
	return s.FileService
}
