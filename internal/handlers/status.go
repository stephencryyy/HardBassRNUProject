package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"BASProject/internal/services"
	"github.com/gorilla/mux"
)

type StatusHandler struct {
	SessionService services.ISessionService
}

func NewStatusHandler(sessionService services.ISessionService) *StatusHandler {
	return &StatusHandler{
		SessionService: sessionService,
	}
}

func (h *StatusHandler) GetUploadStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["session_id"]
	if sessionID == "" {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Missing session_id in URL.", nil, "")
		return
	}

	status, err := h.SessionService.GetUploadStatus(sessionID)
	if err != nil {
		if errors.Is(err, services.ErrSessionNotFound) {
			sendErrorResponse(w, http.StatusNotFound, 404, "Upload session not found.", map[string]interface{}{
				"session_id": sessionID,
			}, "Ensure that the session ID is correct or restart the upload.")
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Internal server error.", err.Error(), "Please try again later.")
		return
	}

	// Формируем ответ на основе полученного статуса
	response := map[string]interface{}{
		"status":          "success",
		"session_id":      sessionID,
		"uploaded_chunks": status["uploaded_chunks"],
		"pending_chunks":  status["pending_chunks"],
		"total_chunks":    status["total_chunks"],
		"message":         status["message"],
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
