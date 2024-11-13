package handlers

import (
	"encoding/json"
	"net/http"

	"BASProject/internal/services"
	"github.com/gorilla/mux"
)

type DeleteHandler struct {
	SessionService *services.SessionService
}

func NewDeleteHandler(sessionService *services.SessionService) *DeleteHandler {
	return &DeleteHandler{
		SessionService: sessionService,
	}
}

func (h *DeleteHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["session_id"]
	if sessionID == "" {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Missing session_id in URL.", nil, "")
		return
	}

	err := h.SessionService.DeleteSession(sessionID)
	if err != nil {
		if err == services.ErrSessionNotFound {
			sendErrorResponse(w, http.StatusNotFound, 404, "Upload session not found.", map[string]interface{}{
				"session_id": sessionID,
			}, "")
			return
		}
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to delete the upload session. Please try again later.", err.Error(), "")
		return
	}

	response := map[string]interface{}{
		"status":     "success",
		"session_id": sessionID,
		"message":    "Upload session deleted successfully.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
