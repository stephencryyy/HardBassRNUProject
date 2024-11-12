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
	"sort"
	"strconv"
	"strings"
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
func (f *FileService) CalculateChunkSize(fileSize, MaxChunkSize int64) int64 {
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

	//tODO: Сохранение чанка на диск
	log.Printf("Saving chunk %d for session %s", chunkID, sessionID)

	// Логика сохранения чанка на диск
	filePath := filepath.Join("./data", fmt.Sprintf("%s_%d.part", sessionID, chunkID))

	// Создаем файл и записываем данные
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create chunk file: %w", err)
	}
	defer file.Close()

	_, err = file.Write(chunkData)
	if err != nil {
		return fmt.Errorf("failed to write chunk data to file: %w", err)
	}
	log.Printf("Chunk %d for session %s saved successfully at %s", chunkID, sessionID, filePath)

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

func (fs *FileService) AssembleChunks(sessionID string, outputFilePath string) error {
	// Открываем файл для записи результата
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// Получаем все файлы чанков для сессии
	chunkFiles, err := filepath.Glob(fmt.Sprintf("./data/%s_*.part", sessionID))
	if err != nil {
		return fmt.Errorf("failed to find chunk files: %w", err)
	}

	if len(chunkFiles) == 0 {
		return fmt.Errorf("no chunks found for session %s", sessionID)
	}

	// Сортируем файлы по chunkID
	sort.Slice(chunkFiles, func(i, j int) bool {
		// Извлекаем chunkID из имени файла
		id1 := extractChunkID(chunkFiles[i])
		id2 := extractChunkID(chunkFiles[j])
		return id1 < id2
	})

	// Собираем все чанки в один файл
	for _, chunkFile := range chunkFiles {
		err := appendChunk(outputFile, chunkFile)
		if err != nil {
			return fmt.Errorf("failed to append chunk %s: %w", chunkFile, err)
		}
	}

	return nil
}

// Вспомогательная функция для извлечения chunkID из имени файла
func extractChunkID(fileName string) int {
	parts := strings.Split(fileName, "_")
	if len(parts) < 2 {
		return 0 // Возвращаем 0, если формат имени файла некорректный
	}
	chunkIDStr := strings.TrimSuffix(parts[len(parts)-1], ".part")
	chunkID, err := strconv.Atoi(chunkIDStr)
	if err != nil {
		return 0 // Возвращаем 0, если преобразование не удалось
	}
	return chunkID
}

// Вспомогательная функция для записи чанка в выходной файл
func appendChunk(outputFile *os.File, chunkFilePath string) error {
	chunkFile, err := os.Open(chunkFilePath)
	if err != nil {
		return fmt.Errorf("failed to open chunk file %s: %w", chunkFile, err)
	}
	defer chunkFile.Close()

	_, err = io.Copy(outputFile, chunkFile)
	if err != nil {
		return fmt.Errorf("failed to write chunk data %s: %w", chunkFile, err)
	}

	return nil
}
