package handlers

import (
	"BASProject/internal/services"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type UploadChunkHandler struct {
	SessionService services.ISessionService
	MaxChunkSize   int
}

func NewUploadChunkHandler(sessionService services.ISessionService) *UploadChunkHandler {
	return &UploadChunkHandler{
		SessionService: sessionService,
	}
}

func (h *UploadChunkHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	// Извлечение session_id из URL
	vars := mux.Vars(r)
	sessionID := vars["session_id"]
	log.Printf("Received session_id: %s", sessionID)

	if sessionID == "" {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Missing session_id in URL.", nil, "")
		return
	}

	chunkIDStr := r.FormValue("chunk_id")
	chunkID, err := strconv.Atoi(chunkIDStr)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Invalid chunk_id format.", nil, "")
		return
	}

	checksum := r.FormValue("checksum")
	if checksum == "" {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Missing checksum.", nil, "")
		return
	}

	chunkFile, _, err := r.FormFile("chunk_data")
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Error reading chunk data.", nil, "")
		log.Println(err)
		return
	}
	defer chunkFile.Close()

	// Используем Content-Length для определения размера текущего чанка
	chunkSize := r.ContentLength

	// Вычисляем таймаут в зависимости от размера чанка
	timeout := time.Duration(10*chunkSize/1024/1024) * time.Second
	if timeout < 60*time.Second { // Минимальный таймаут — 60 секунд
		timeout = 60 * time.Second
	}

	// Контекст с динамическим таймаутом
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Используем каналы для асинхронного чтения данных с таймаутом
	fileDataChan := make(chan []byte)
	errChan := make(chan error)

	go func() {
		fileData, err := io.ReadAll(chunkFile)
		if err != nil {
			errChan <- err
			return
		}
		fileDataChan <- fileData
	}()

	var fileData []byte
	select {
	case fileData = <-fileDataChan:
		log.Printf("Read chunk of size: %d bytes", len(fileData))
	case err = <-errChan:
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to read chunk data.", err.Error(), "")
		return
	case <-ctx.Done():
		// Время ожидания истекло — возвращаем сообщение об ошибке
		sendErrorResponse(w, http.StatusGatewayTimeout, 504, fmt.Sprintf("Timeout processing chunk. Chunk size: %d bytes, timeout: %.0f seconds.", chunkSize, timeout.Seconds()), nil, "Please try uploading the chunk again.")
		return
	}

	// Проверка контрольной суммы
	isValidChecksum := h.SessionService.GetFileService().ValidateChecksum(fileData, checksum)
	log.Printf("Checksum valid: %v", isValidChecksum)
	if !isValidChecksum {
		sendErrorResponse(w, http.StatusPreconditionFailed, 412, "Checksum validation failed.", map[string]interface{}{
			"expected_checksum": checksum,
			"provided_checksum": h.SessionService.GetFileService().CalculateChecksum(fileData),
		}, "Please resend the chunk with the correct data.")
		return
	}

	// Проверка, существует ли уже чанк на сервере
	exists, err := h.SessionService.GetFileService().ChunkExists(sessionID, chunkID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Error checking chunk existence.", err.Error(), "")
		return
	}
	if exists {
		sendErrorResponse(w, http.StatusConflict, 409, "Chunk already uploaded.", map[string]interface{}{
			"chunk_id":   chunkID,
			"session_id": sessionID,
		}, "Check uploaded chunks via /upload/status before sending.")
		return
	}

	// Сохраняем чанк
	err = h.SessionService.GetFileService().SaveChunk(sessionID, chunkID, fileData)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Internal server error.", err.Error(), "Please try again later.")
		return
	}

	nextChunkID := chunkID + 1

	// Ответ о успешной загрузке чанка
	log.Printf("Chunk %d uploaded successfully. Next chunk_id: %d", chunkID, nextChunkID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "success",
		"message":       fmt.Sprintf("Chunk %d uploaded successfully.", chunkID),
		"next_chunk_id": nextChunkID,
	})
}

func sendErrorResponse(w http.ResponseWriter, statusCode int, errorCode int, message string, details interface{}, suggestion string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "error",
		"error_code": errorCode,
		"message":    message,
		"details":    details,
		"suggestion": suggestion,
	})
}
