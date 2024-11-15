package test

import (
	"BASProject/internal/services"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"BASProject/internal/handlers"
)

// Test для проверки отсутствия session_id
func TestDeleteSession_MissingSessionID(t *testing.T) {
	mockService := &services.SessionServiceMock{}
	handler := handlers.NewDeleteHandler(mockService)

	req, err := http.NewRequest("DELETE", "/delete", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	handler.DeleteSession(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Missing session_id in URL.", response["message"])
}

// Test для проверки отсутствующей сессии
func TestDeleteSession_SessionNotFound(t *testing.T) {
	mockService := &services.SessionServiceMock{
		DeleteSessionFunc: func(sessionID string) error {
			return services.ErrSessionNotFound
		},
	}
	handler := handlers.NewDeleteHandler(mockService)

	req, err := http.NewRequest("DELETE", "/delete/session123", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"session_id": "session123"})
	handler.DeleteSession(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Upload session not found.", response["message"])
	assert.Equal(t, "session123", response["details"].(map[string]interface{})["session_id"])
}

// Test для проверки внутренней ошибки сервера
func TestDeleteSession_InternalServerError(t *testing.T) {
	mockService := &services.SessionServiceMock{
		DeleteSessionFunc: func(sessionID string) error {
			return errors.New("some internal error")
		},
	}
	handler := handlers.NewDeleteHandler(mockService)

	req, err := http.NewRequest("DELETE", "/delete/session123", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"session_id": "session123"})
	handler.DeleteSession(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Failed to delete the upload session. Please try again later.", response["message"])
}

// Test для успешного удаления сессии
func TestDeleteSession_Success(t *testing.T) {
	mockService := &services.SessionServiceMock{
		DeleteSessionFunc: func(sessionID string) error {
			return nil
		},
	}
	handler := handlers.NewDeleteHandler(mockService)

	req, err := http.NewRequest("DELETE", "/delete/session123", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"session_id": "session123"})
	handler.DeleteSession(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Upload session deleted successfully.", response["message"])
	assert.Equal(t, "session123", response["session_id"])
}
