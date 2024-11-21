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

// Обработчик завершения загрузки
func (h *UploadChunkHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	// Получаем session_id из URL
	vars := mux.Vars(r)
	sessionID := vars["session_id"]
	log.Printf("Received session_id: %s", sessionID)

	if sessionID == "" {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Missing session_id in URL.", nil, "")
		return
	}

	// Получаем статус загрузки сессии
	status, err := h.SessionService.GetUploadStatus(sessionID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to get upload status.", err.Error(), "")
		return
	}

	// Проверяем, все ли чанки загружены
	if status["completed"] == false {
		missingChunks := status["pending_chunks"].([]int)
		sendErrorResponse(w, http.StatusConflict, 409, "File upload incomplete. Some chunks are still missing.", map[string]interface{}{
			"missing_chunks": missingChunks,
		}, "Upload the missing chunks before completing the session.")
		return
	}

	// Получаем имя файла из сессии
	fileName, ok := status["file_name"].(string)
	if !ok || fileName == "" {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "File name missing in session data.", nil, "")
		return
	}

	// Убедимся, что папка для загрузок существует сохраняем куда угодно через флаг
	uploadDir := "./data"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to create uploads directory.", err.Error(), "")
		return
	}

	// Генерируем уникальное имя файла
	uniqueFileName, err := GenerateUniqueFileName(uploadDir, fileName)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "File name nothing .", err.Error(), "")
		return
	}
	outputPath := filepath.Join(uploadDir, uniqueFileName)

	// Все чанки загружены, теперь собираем файл
	err = h.SessionService.GetFileService().AssembleChunks(sessionID, outputPath)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to assemble chunks.", err.Error(), "")
		return
	}

	// Завершаем сессию
	err = h.SessionService.UpdateProgress(sessionID, status["file_size"].(int64))
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to update session progress.", err.Error(), "")
		return
	}

	// Удаляем временные файлы
	err = h.SessionService.GetFileService().DeleteChunks(sessionID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to delete session.", err.Error(), "")
		return
	}

	// Ответ об успешном завершении
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"session_id":  sessionID,
		"message":     "File upload completed successfully.",
		"output_file": outputPath,
	})
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
