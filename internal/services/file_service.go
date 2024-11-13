package services

import (
	"fmt"
	"os"
	"path/filepath"
)

func (s *FileService) DeleteChunks(sessionID string) error {
	// Получаем список файлов чанков в директории
	pattern := fmt.Sprintf("%s_%s", sessionID, "*.part")
	filesPattern := filepath.Join(s.LocalPath, pattern)
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

	// Удаляем собранный файл (если он существует)
	assembledFileName := fmt.Sprintf("%s", sessionID) // Зависит от вашей реализации
	assembledFilePath := filepath.Join(s.LocalPath, assembledFileName)
	if _, err := os.Stat(assembledFilePath); err == nil {
		err := os.Remove(assembledFilePath)
		if err != nil {
			return fmt.Errorf("failed to delete assembled file %s: %w", assembledFilePath, err)
		}
	}

	return nil
}
