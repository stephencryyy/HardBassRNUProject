package test

import (
	"BASProject/internal/handlers"
	"BASProject/internal/services"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test для проверки неверного JSON-формата
func TestStartSession_InvalidJSON(t *testing.T) {
	mockService := &services.SessionServiceMock{}
	handler := handlers.NewStartHandler(mockService)

	req, err := http.NewRequest("POST", "/start", bytes.NewBuffer([]byte("{invalid_json}")))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler.StartSession(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid JSON format")
}

// Test для проверки отсутствия параметров
func TestStartSession_MissingParameters(t *testing.T) {
	mockService := &services.SessionServiceMock{}
	handler := handlers.NewStartHandler(mockService)

	requestBody, _ := json.Marshal(map[string]interface{}{})
	req, err := http.NewRequest("POST", "/start", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler.StartSession(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Invalid request. Missing or incorrect parameters.", response["message"])
}

// Test для успешного создания сессии
func TestStartSession_Success(t *testing.T) {
	mockService := &services.SessionServiceMock{
		CreateSessionFunc: func(fileName string, fileSize int64, fileHash string) (int64, error) {
			return 1024, nil
		},
	}
	handler := handlers.NewStartHandler(mockService)

	requestBody, _ := json.Marshal(map[string]interface{}{
		"file_name": "testfile",
		"file_size": 2048,
	})
	req, err := http.NewRequest("POST", "/start", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler.StartSession(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "test-session-id", response["session_id"])
	assert.Equal(t, float64(1024), response["chunk_size"]) // JSON unmarshalling возвращает числа как float64
}

// Test для проверки ошибки от SessionService
func TestStartSession_ServiceError(t *testing.T) {
	mockService := &services.SessionServiceMock{
		CreateSessionFunc: func(fileName string, fileSize int64, fileHash string) (int64, error) {
			return 0, errors.New("service error")
		},
	}
	handler := handlers.NewStartHandler(mockService)

	requestBody, _ := json.Marshal(map[string]interface{}{
		"file_name": "testfile",
		"file_size": 2048,
	})
	req, err := http.NewRequest("POST", "/start", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler.StartSession(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Internal server error.", response["message"])
}
