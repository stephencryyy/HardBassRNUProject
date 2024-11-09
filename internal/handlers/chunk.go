package handlers

import (
	"BASProject/internal/services"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type UploadChunkHandler struct {
	SessionService *services.SessionService
	MaxChunkSize   int
}

func NewUploadChunkHandler(sessionService *services.SessionService) *UploadChunkHandler {
	return &UploadChunkHandler{
		SessionService: sessionService,
	}
}

func (h *UploadChunkHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	// Логируем получение session_id
	vars := mux.Vars(r)
	sessionID := vars["session_id"]
	log.Printf("Received session_id: %s", sessionID)

	if sessionID == "" {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Missing session_id in URL.", nil, "")
		return
	}

	err := r.ParseMultipartForm(10 * 1024 * 1024)
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

	fileData, err := io.ReadAll(chunkFile)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to read chunk data.", err.Error(), "")
		return
	}

	log.Printf("Read chunk data of size: %d bytes", len(fileData))

	// Проверка контрольной суммы через FileService
	isValidChecksum := h.SessionService.FileService.ValidateChecksum(fileData, checksum)
	log.Printf("Checksum valid: %v", isValidChecksum)
	if !isValidChecksum {
		sendErrorResponse(w, http.StatusPreconditionFailed, 412, "Checksum validation failed.", map[string]interface{}{
			"expected_checksum": checksum,
			"provided_checksum": h.SessionService.FileService.CalculateChecksum(fileData),
		}, "Please resend the chunk with the correct data.")
		return
	}

	err = h.SessionService.FileService.SaveChunk(sessionID, chunkID, fileData)
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

	nextChunkID := chunkID + 1

	// Успешный ответ с логом
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
