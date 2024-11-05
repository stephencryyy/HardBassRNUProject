package handlers

import (
	"encoding/json"
	"net/http"

	"BASProject/internal/services"
)

type StartHandler struct {
	SessionService *services.SessionService
}

func NewStartHandler(sessionService *services.SessionService) *StartHandler {
	return &StartHandler{
		SessionService: sessionService,
	}
}

func (h *StartHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		FileName string `json:"file_name"`
		FileSize int64  `json:"file_size"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	if requestData.FileName == "" || requestData.FileSize <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "error",
			"error_code": 400,
			"message":    "Invalid request. Missing or incorrect parameters.",
			"details":    "Parameter 'file_size' is required and must be a positive integer.",
		})
		return
	}

	sessionID, chunkSize, err := h.SessionService.CreateSession(requestData.FileName, requestData.FileSize)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "error",
			"error_code": 500,
			"message":    "Internal server error.",
			"details":    err.Error(),
		})
		return
	}

	responseData := map[string]interface{}{
		"session_id": sessionID,
		"chunk_size": chunkSize,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responseData)
}
