package handlers

import (
	"BASProject/internal/services"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"strconv"
)

type UploadChunkHandler struct {
	SessionService *services.SessionService
}

func NewUploadChunkHandler(sessionService *services.SessionService) *UploadChunkHandler {
	return &UploadChunkHandler{
		SessionService: sessionService,
	}
}

func (h *UploadChunkHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	// Получаем session_id из URL
	vars := mux.Vars(r)
	sessionID := vars["session_id"]
	if sessionID == "" {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Missing session_id in URL.", nil, "")
		return
	}

	// Чтение данных из multipart
	err := r.ParseMultipartForm(10 * 1024 * 1024) // Максимальный размер формы 10MB
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Error parsing multipart form.", nil, "")
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
		return
	}
	defer chunkFile.Close()

	// Читаем файл в []byte
	fileData, err := io.ReadAll(chunkFile)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to read chunk data.", err.Error(), "")
		return
	}

	// Проверка размера чанка
	maxChunkSize := 4 * 1024 * 1024 // 4MB
	if len(fileData) > maxChunkSize {
		sendErrorResponse(w, http.StatusRequestEntityTooLarge, 413, "Chunk size exceeds the allowed limit.", map[string]interface{}{
			"max_chunk_size":      maxChunkSize,
			"provided_chunk_size": len(fileData),
		}, "Reduce the chunk size and resend.")
		return
	}

	// Вычисляем контрольную сумму для чанка
	isValidChecksum := h.SessionService.ChecksumService.ValidateChecksum(fileData, checksum)
	if !isValidChecksum {
		sendErrorResponse(w, http.StatusPreconditionFailed, 412, "Checksum validation failed.", map[string]interface{}{
			"expected_checksum": checksum,
			"provided_checksum": h.SessionService.ChecksumService.CalculateChecksum(fileData),
		}, "Please resend the chunk with the correct data.")
		return
	}

	// Сохраняем чанк
	err = h.SessionService.SaveChunk(sessionID, chunkID, fileData)
	if err != nil {
		if err == services.ErrChunkAlreadyExists {
			sendErrorResponse(w, http.StatusConflict, 409, "Chunk already uploaded.", map[string]interface{}{
				"chunk_id":   chunkID,
				"session_id": sessionID,
			}, "Check uploaded chunks via /upload/status before sending.")
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Internal server error.", err.Error(), "Please try again later.")
		return
	}

	// Ответ на успешную загрузку
	nextChunkID := chunkID + 1

	// Ответ клиенту
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
