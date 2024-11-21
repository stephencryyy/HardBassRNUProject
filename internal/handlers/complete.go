package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

// Обработчик завершения загрузки
func (h *UploadChunkHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	// Get session_id from URL
	vars := mux.Vars(r)
	sessionID := vars["session_id"]
	log.Printf("Received session_id: %s", sessionID)

	if sessionID == "" {
		sendErrorResponse(w, http.StatusBadRequest, 400, "Missing session_id in URL.", nil, "")
		return
	}

	// Update upload progress before checking status
	err := h.SessionService.UpdateProgress(sessionID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to update upload progress.", err.Error(), "")
		return
	}

	// Get session data
	status, err := h.SessionService.GetUploadStatus(sessionID)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to get upload status.", err.Error(), "")
		return
	}

	// Extract and validate 'completed' status
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

	// Retrieve and assert 'status' string
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

	// Retrieve and assert 'file_name'
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

	// Specify the output file path
	outputFilePath := filepath.Join("./uploads", fileName)

	// Ensure the uploads directory exists
	if err := os.MkdirAll("./uploads", os.ModePerm); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, 500, "Failed to create uploads directory.", err.Error(), "")
		return
	}

	// Assemble the file
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
		return
	}

	// Return success response
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
