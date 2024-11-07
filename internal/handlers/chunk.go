package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"crypto/sha256"
	"encoding/hex"
	"BASProject/internal/services"
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
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "Missing session_id", http.StatusBadRequest)
		return
	}

	// Чтение данных из multipart
	err := r.ParseMultipartForm(10 * 1024 * 1024) // Максимальный размер формы 10MB
	if err != nil {
		http.Error(w, "Error parsing multipart form", http.StatusBadRequest)
		return
	}

	chunkIDStr := r.FormValue("chunk_id")
	chunkID, err := strconv.Atoi(chunkIDStr)
	if err != nil {
		http.Error(w, "Invalid chunk_id format", http.StatusBadRequest)
		return
	}

	checksum := r.FormValue("checksum")
	chunkData, _, err := r.FormFile("chunk_data")
	if err != nil {
		http.Error(w, "Error reading chunk data", http.StatusBadRequest)
		return
	}
	defer chunkData.Close()

	// Читаем файл в []byte
	fileData, err := io.ReadAll(chunkData)
	if err != nil {
		http.Error(w, "Failed to read chunk data", http.StatusInternalServerError)
		return
	}

	// Вычисляем контрольную сумму для чанка
	hash := sha256.New()
	_, err = hash.Write(fileData)
	if err != nil {
		http.Error(w, "Error calculating checksum", http.StatusInternalServerError)
		return
	}
	calculatedChecksum := hex.EncodeToString(hash.Sum(nil))

	// Проверка контрольной суммы
	if checksum != calculatedChecksum {
		w.WriteHeader(http.StatusPreconditionFailed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":             "error",
			"error_code":         412,
			"message":            "Checksum validation failed.",
			"expected_checksum":  checksum,
			"provided_checksum":  calculatedChecksum,
			"suggestion":         "Please resend the chunk with the correct data.",
		})
		return
	}

	// Сохраняем чанк
	err = h.SessionService.SaveChunk(sessionID, chunkID, fileData)
	if err != nil {
		http.Error(w, "Failed to save chunk", http.StatusInternalServerError)
		return
	}

	// Ответ на успешную загрузку
	nextChunkID, err := h.SessionService.GetNextChunkID(sessionID)
	if err != nil {
		http.Error(w, "Failed to retrieve next chunk ID", http.StatusInternalServerError)
		return
	}

	// Ответ клиенту
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "success",
		"message":       fmt.Sprintf("Chunk %d uploaded successfully.", chunkID),
		"next_chunk_id": nextChunkID,
	})
}
