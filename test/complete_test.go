package test

import (
	"BASProject/internal/handlers"
	"BASProject/internal/services"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestCompleteUpload_MissingSessionID(t *testing.T) {
	mockService := &services.SessionServiceMock{
		CreateSessionFunc: func(fileName string, fileSize int64, fileHash string) (int64, error) {
			return 1024, nil
		},
	}

	handler := handlers.NewUploadChunkHandler(mockService)
	req, err := http.NewRequest("POST", "/complete", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/complete/{session_id}", handler.CompleteUpload)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Missing session_id in URL.", response["message"])
}

func TestCompleteUpload_IncompleteUpload(t *testing.T) {
	mockService := &services.SessionServiceMock{
		GetUploadStatusFunc: func(sessionID string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"completed":      false,
				"pending_chunks": []int{2, 3},
			}, nil
		},
	}

	handler := handlers.NewUploadChunkHandler(mockService)
	req, err := http.NewRequest("POST", "/complete/session123", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/complete/{session_id}", handler.CompleteUpload)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "File upload incomplete. Some chunks are still missing.", response["message"])
	assert.Equal(t, []interface{}{2, 3}, response["details"].(map[string]interface{})["missing_chunks"])
}

func TestCompleteUpload_AssembleChunksError(t *testing.T) {
	mockService := &services.SessionServiceMock{
		GetUploadStatusFunc: func(sessionID string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"completed": true,
				"file_size": int64(1024),
			}, nil
		},
		FileService: &services.FileServiceMock{
			AssembleChunksFunc: func(sessionID, outputFilePath string) error {
				return fmt.Errorf("assembly error")
			},
		},
	}

	handler := handlers.NewUploadChunkHandler(mockService)
	req, err := http.NewRequest("POST", "/complete/session123", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/complete/{session_id}", handler.CompleteUpload)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Failed to assemble chunks.", response["message"])
}

func TestCompleteUpload_Success(t *testing.T) {
	mockService := &services.SessionServiceMock{
		GetUploadStatusFunc: func(sessionID string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"completed": true,
				"file_size": int64(1024),
			}, nil
		},
		UpdateProgressFunc: func(sessionID string) error {
			return nil
		},
		FileService: &services.FileServiceMock{
			AssembleChunksFunc: func(sessionID, outputFilePath string) error {
				return nil
			},
		},
	}

	handler := handlers.NewUploadChunkHandler(mockService)
	req, err := http.NewRequest("POST", "/complete/session123", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/complete/{session_id}", handler.CompleteUpload)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "File upload completed successfully.", response["message"])
}
