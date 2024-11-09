package services

import (
	"BASProject/internal/storage"
	"BASProject/internal/utils"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

type FileService struct {
	Storage         *storage.RedisClient
	ChecksumService *utils.ChecksumService
	LocalPath       string
}

func NewFileService(storage *storage.RedisClient, localPath string) *FileService {
	return &FileService{
		Storage:         storage,
		ChecksumService: utils.NewChecksumService(),
		LocalPath:       localPath,
	}
}

// Проверка существования файла на сервере
func (f *FileService) FileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return !errors.Is(err, os.ErrNotExist)
}

// Вычисление подходящего размера чанка в зависимости от размера файла
func (f *FileService) CalculateChunkSize(fileSize int64) int64 {
	if fileSize < 50*1024*1024 { // Меньше 50MB
		return 4 * 1024 * 1024 // 4MB
	} else if fileSize < 500*1024*1024 { // Меньше 500MB
		return 8 * 1024 * 1024 // 8MB
	}
	return 16 * 1024 * 1024 // Больше 500MB
}

// Сохранение чанка
func (f *FileService) SaveChunk(sessionID string, chunkID int, chunkData []byte) error {
	// Проверка, загружен ли чанк
	chunkExists, err := f.Storage.ChunkExists(sessionID, chunkID)
	if err != nil {
		return fmt.Errorf("failed to check if chunk exists: %w", err)
	}
	if chunkExists {
		return ErrChunkAlreadyExists
	}

	// Сохранение чанка в Redis
	err = f.Storage.SaveChunkData(sessionID, chunkID, chunkData)
	if err != nil {
		return fmt.Errorf("failed to save chunk data: %w", err)
	}

	// Обновление общего прогресса загрузки
	err = f.Storage.UpdateUploadedSize(sessionID, int64(len(chunkData)))
	if err != nil {
		return fmt.Errorf("failed to update uploaded size: %w", err)
	}

	return nil
}

// Метод для получения следующего ID чанка
func (f *FileService) GetNextChunkID(sessionID string) (int, error) {
	// Получаем список всех чанков для данной сессии из Redis
	chunks, err := f.Storage.GetChunks(sessionID)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve chunks: %w", err)
	}

	// Ищем максимальный существующий ID чанка
	maxChunkID := -1
	for _, chunkID := range chunks {
		if chunkID > maxChunkID {
			maxChunkID = chunkID
		}
	}

	// Следующий доступный ID будет на 1 больше максимального
	return maxChunkID + 1, nil
}

var ErrChunkAlreadyExists = errors.New("chunk already exists")

// ValidateChecksum проверяет контрольную сумму данных.
func (f *FileService) ValidateChecksum(data []byte, expectedChecksum string) bool {
	calculatedChecksum := f.CalculateChecksum(data)
	return calculatedChecksum == expectedChecksum
}

// CalculateChecksum вычисляет контрольную сумму данных в формате SHA-256.
func (f *FileService) CalculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
