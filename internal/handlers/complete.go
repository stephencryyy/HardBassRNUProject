package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

func (h *UploadChunkHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {

	// Получаем session_id из URL
	vars := mux.Vars(r)
	sessionID := vars["session_id"]
	log.Printf("Received session_id: %s", sessionID)

	if sessionID == "" {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Missing session_id in URL.", nil, "")
		return
	}

	// Обновляем прогресс загрузки перед проверкой статуса
	err := h.SessionService.UpdateProgress(sessionID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to update upload progress.", err.Error(), "")
		return
	}

	// Получаем данные сессии
	status, err := h.SessionService.GetUploadStatus(sessionID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to get upload status.", err.Error(), "")
		return
	}

	// Извлекаем и проверяем статус 'completed'
	completedInterface, ok := status["completed"]
	if !ok {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Missing 'completed' status.", nil, "")
		return
	}
	completed, ok := completedInterface.(bool)
	if !ok {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Invalid 'completed' status type.", nil, "")
		return
	}

	// Извлекаем и проверяем строку 'status'
	statusInterface, ok := status["status"]
	if !ok {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Missing 'status'.", nil, "")
		return
	}
	statusStr, ok := statusInterface.(string)
	if !ok {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Invalid 'status' type.", nil, "")
		return
	}

	if !completed || statusStr != "completed" {
		h.cleanupSession(sessionID)
		sendErrorResponse(w, http.StatusConflict, 409, "Upload incomplete or session is in progress. Session data has been cleaned up.", nil, "")
		return
	}

	// Извлекаем и проверяем 'file_name'
	fileNameInterface, ok := status["file_name"]
	if !ok {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Missing 'file_name'.", nil, "")
		return
	}
	fileName, ok := fileNameInterface.(string)
	if !ok || fileName == "" {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Invalid 'file_name'.", nil, "")
		return
	}

	// Получаем путь для сборки полного файла из FileService через интерфейсный метод
	storagePath, err := h.SessionService.GetFileService().GetStoragePath()
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to retrieve storage path.", nil, "")
		return
	}

	// Генерируем уникальное имя файла
	uniqueFileName, err := GenerateUniqueFileName(storagePath, fileName)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to generate unique file name.", nil, "")
		return
	}

	// Задаём путь к выходному файлу
	outputFilePath := filepath.Join(storagePath, uniqueFileName)

	// Убеждаемся, что директория для загрузок существует
	if err := os.MkdirAll(storagePath, os.ModePerm); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to create uploads directory.", err.Error(), "")
		return
	}

	// Собираем файл
	err = h.SessionService.GetFileService().AssembleChunks(sessionID, outputFilePath)
	if err != nil {
		h.cleanupSession(sessionID)
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to assemble chunks. Session data has been cleaned up.", err.Error(), "")
		return
	}

	// Удаляем файлы чанков
	err = h.SessionService.GetFileService().DeleteChunks(sessionID)
	if err != nil {
		log.Printf("Failed to delete chunks for session %s: %v", sessionID, err)
	}

	// Возвращаем успешный ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"session_id": sessionID,
		"message":    "File upload completed successfully.",
	})
}

func (h *UploadChunkHandler) cleanupSession(sessionID string) {
	// Удаляем файлы чанков
	err := h.SessionService.GetFileService().DeleteChunks(sessionID)
	if err != nil {
		log.Printf("Failed to delete chunks for session %s: %v", sessionID, err)
	}

	// Удаляем данные сессии из Redis
	err = h.SessionService.DeleteSession(sessionID)
	if err != nil {
		log.Printf("Failed to delete session data for session %s: %v", sessionID, err)
	}
}

// GenerateUniqueFileName проверяет, существует ли файл, и добавляет суффикс, если нужно.
func GenerateUniqueFileName(directory, fileName string) (string, error) {
	// Разделяем имя файла и расширение
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	extension := filepath.Ext(fileName)
	uniqueName := fileName
	counter := 1

	for {
		// Формируем путь к файлу
		filePath := filepath.Join(directory, uniqueName)

		// Проверяем, существует ли файл
		_, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			// Файл не существует, можно использовать это имя
			return uniqueName, nil
		} else if err != nil {
			// Произошла какая-то другая ошибка при доступе к файлу
			return "", err
		}

		// Если файл существует, добавляем суффикс и проверяем снова
		uniqueName = fmt.Sprintf("%s(%d)%s", baseName, counter, extension)
		counter++
	}
}
