package handlers

import (
	"encoding/json"
	"log"
	"net/http"

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

	// Все чанки загружены, теперь собираем файл
	err = h.SessionService.GetFileService().AssembleChunks(sessionID, "target_file_name") // Укажите имя целевого файла
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

	// временные файлы
	err = h.SessionService.GetFileService().DeleteChunks(sessionID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to delete session.", err.Error(), "")
		return
	}

	// Ответ об успешном завершении
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"session_id": sessionID,
		"message":    "File upload completed successfully.",
	})
}
