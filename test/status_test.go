package test

import (
	"BASProject/internal/handlers"
	"BASProject/internal/services"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// Test для проверки отсутствия session_id
func TestGetUploadStatus_MissingSessionID(t *testing.T) {
	mockService := &services.SessionServiceMock{}
	handler := handlers.NewStatusHandler(mockService)

	req, err := http.NewRequest("GET", "/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	handler.GetUploadStatus(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// Test для проверки отсутствующей сессии
func TestGetUploadStatus_SessionNotFound(t *testing.T) {
	mockService := &services.SessionServiceMock{
		GetUploadStatusFunc: func(sessionID string) (map[string]interface{}, error) {
			return nil, services.ErrSessionNotFound
		},
	}
	handler := handlers.NewStatusHandler(mockService)

	req, err := http.NewRequest("GET", "/status/session123", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	// Устанавливаем переменные в mux, как если бы `session_id` был извлечен из URL
	req = mux.SetURLVars(req, map[string]string{"session_id": "session123"})
	handler.GetUploadStatus(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Upload session not found.", response["message"])
	assert.Equal(t, "session123", response["details"].(map[string]interface{})["session_id"])
}

// Test для проверки внутренней ошибки сервера
func TestGetUploadStatus_InternalServerError(t *testing.T) {
	mockService := &services.SessionServiceMock{
		GetUploadStatusFunc: func(sessionID string) (map[string]interface{}, error) {
			return nil, errors.New("some internal error")
		},
	}
	handler := handlers.NewStatusHandler(mockService)

	req, err := http.NewRequest("GET", "/status/session123", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"session_id": "session123"})
	handler.GetUploadStatus(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Internal server error.", response["message"])
}

// Test для успешного получения статуса
func TestGetUploadStatus_Success(t *testing.T) {
	mockService := &services.SessionServiceMock{
		GetUploadStatusFunc: func(sessionID string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"uploaded_chunks": 5,
				"pending_chunks":  []int{6, 7, 8},
				"total_chunks":    8,
				"message":         "Upload in progress",
			}, nil
		},
	}
	handler := handlers.NewStatusHandler(mockService)

	req, err := http.NewRequest("GET", "/status/session123", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"session_id": "session123"})
	handler.GetUploadStatus(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Upload in progress", response["message"])
	assert.Equal(t, "session123", response["session_id"])
	assert.Equal(t, float64(5), response["uploaded_chunks"]) // JSON unmarshal возвращает числа как float64
	assert.Equal(t, float64(8), response["total_chunks"])
}
