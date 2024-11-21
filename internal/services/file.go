package services

import (
	"BASProject/internal/storage"
	"BASProject/internal/utils"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type FileService struct {
	Storage         *storage.RedisClient
	ChecksumService *utils.ChecksumService
	LocalPath       string
}
type IFileService interface {
	FileExists(fileName string) bool
	CalculateChunkSize(fileSize, MaxChunkSize int64) int64
	SaveChunk(sessionID string, chunkID int, chunkData []byte) error
	GetNextChunkID(sessionID string) (int, error)
	ValidateChecksum(chunkData []byte, expectedChecksum string) bool
	CalculateChecksum(chunkData []byte) string
	AssembleChunks(sessionID string, outputFilePath string) error
	DeleteChunks(sessionID string) error
	ChunkExists(sessionID string, chunkID int) (bool, error)
	GetStoragePath() (string, error)
}

func NewFileService(storage *storage.RedisClient, localPath string) *FileService {
	if localPath == "" {
		localPath = "data"
	}
	return &FileService{
		Storage:         storage,
		ChecksumService: utils.NewChecksumService(),
		LocalPath:       localPath,
	}
}

// Проверка существования файла на сервере
func (f *FileService) FileExists(fileName string) bool {
	filePath := filepath.Join(f.LocalPath, fileName)
	_, err := os.Stat(filePath)
	return !errors.Is(err, os.ErrNotExist)
}

// Вычисление подходящего размера чанка в зависимости от размера файла
func (f *FileService) CalculateChunkSize(fileSize, MaxChunkSize int64) int64 {
	log.Printf("fileSize: %d, MaxChunkSize: %d", fileSize, MaxChunkSize)
	chunkSize := int64(0)
	if fileSize < 50*1024*1024 { // Меньше 50MB
		chunkSize = 4 * 1024 * 1024 // 4MB
	} else if fileSize < 500*1024*1024 { // Меньше 500MB
		chunkSize = 8 * 1024 * 1024 // 8MB
	} else {
		chunkSize = 16 * 1024 * 1024 // Больше 500MB
	}
	if MaxChunkSize > 0 && chunkSize > MaxChunkSize {
		chunkSize = MaxChunkSize
	}
	return chunkSize
}

// Сохранение чанка
func (f *FileService) SaveChunk(sessionID string, chunkID int, chunkData []byte) error {
	// Проверяем, существует ли чанк в Redis
	ChunkExists, err := f.Storage.ChunkExists(sessionID, chunkID)
	if err != nil {
		return fmt.Errorf("failed to check chunk existence: %w", err)
	}
	if ChunkExists {
		log.Printf("Chunk %d for session %s already exists. Skipping upload.", chunkID, sessionID)
		return ErrChunkAlreadyExists
	}

	log.Printf("Saving chunk %d for session %s", chunkID, sessionID)
	// Сохраняем чанк на диск
	filePath := filepath.Join(f.LocalPath, fmt.Sprintf("%s_%d.part", sessionID, chunkID))

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create chunk file: %w", err)
	}
	defer file.Close()
	_, err = file.Write(chunkData)
	if err != nil {
		return fmt.Errorf("failed to write chunk chunkData: %w", err)
	}

	// Отмечаем чанк как загруженный в Redis
	err = f.Storage.AddUploadedChunk(sessionID, chunkID)
	if err != nil {
		return fmt.Errorf("failed to mark chunk %d as uploaded: %w", chunkID, err)
	}
	err = f.Storage.UpdateUploadedSize(sessionID, int64(len(chunkData)))
	if err != nil {
		return fmt.Errorf("failed to mark chunk %d as uploaded: %w", chunkID, err)
	}

	log.Printf("Chunk %d for session %s saved successfully.", chunkID, sessionID)
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
func (f *FileService) ValidateChecksum(chunkData []byte, expectedChecksum string) bool {
	calculatedChecksum := f.CalculateChecksum(chunkData)
	return calculatedChecksum == expectedChecksum
}

// CalculateChecksum вычисляет контрольную сумму данных в формате SHA-256.
func (f *FileService) CalculateChecksum(chunkData []byte) string {
	hash := sha256.Sum256(chunkData)
	return hex.EncodeToString(hash[:])
}

// Сборка чанков в итоговый файл
func (fs *FileService) AssembleChunks(sessionID string, outputFilePath string) error {
	sessionData, err := fs.Storage.GetSessionData(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session data: %w", err)
	}

	fileSize, err := extractInt64(sessionData["file_size"])
	if err != nil {
		return fmt.Errorf("invalid file size in session data: %v", err)
	}

	chunkSize, err := extractInt64(sessionData["chunk_size"])
	if err != nil {
		return fmt.Errorf("invalid chunk size in session data: %v", err)
	}

	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)

	missingChunks := []int{}
	for i := 1; i <= totalChunks; i++ {
		chunkFile := filepath.Join(fs.LocalPath, fmt.Sprintf("%s_%d.part", sessionID, i))
		if _, err := os.Stat(chunkFile); os.IsNotExist(err) {
			missingChunks = append(missingChunks, i)
		}
	}

	if len(missingChunks) > 0 {
		return fmt.Errorf("missing chunks: %v", missingChunks)
	}

	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	for i := 1; i <= totalChunks; i++ {
		chunkFile := filepath.Join(fs.LocalPath, fmt.Sprintf("%s_%d.part", sessionID, i))
		err := appendChunk(outputFile, chunkFile)
		if err != nil {
			return fmt.Errorf("failed to append chunk %s: %w", chunkFile, err)
		}
	}

	return nil
}

// Вспомогательная функция для записи чанка в выходной файл
func appendChunk(outputFile *os.File, chunkFilePath string) error {
	chunkFile, err := os.Open(chunkFilePath)
	if err != nil {
		return fmt.Errorf("failed to open chunk file %s: %w", chunkFilePath, err)
	}
	defer chunkFile.Close()

	_, err = io.Copy(outputFile, chunkFile)
	if err != nil {
		return fmt.Errorf("failed to write chunk chunkData from file %s: %w", chunkFilePath, err)
	}

	return nil
}

// Удаление чанков
func (f *FileService) DeleteChunks(sessionID string) error {
	pattern := fmt.Sprintf("%s_*.part", sessionID)
	filesPattern := filepath.Join(f.LocalPath, pattern)
	files, err := filepath.Glob(filesPattern)
	if err != nil {
		return fmt.Errorf("failed to list chunk files: %w", err)
	}

	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			return fmt.Errorf("failed to delete chunk file %s: %w", file, err)
		}
	}
	return nil
}

func (f *FileService) GenerateUniqueName(fileName string) string {
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	extension := filepath.Ext(fileName)

	for i := 1; ; i++ {
		newName := fmt.Sprintf("%s(%d)%s", baseName, i, extension)
		if !f.FileExists(newName) {
			return newName
		}
	}
}

func (f *FileService) ChunkExists(sessionID string, chunkID int) (bool, error) {
	return f.Storage.ChunkExists(sessionID, chunkID)
}

func (f *FileService) GetStoragePath() (string, error) {
	if f.LocalPath == "" {
		return "data", nil
	}
	return f.LocalPath, nil
}
