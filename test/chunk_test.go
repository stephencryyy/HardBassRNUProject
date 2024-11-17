package test

import (
	"BASProject/internal/handlers"
	"BASProject/internal/services"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestUploadChunkHandler_MissingSessionID(t *testing.T) {
	handler := handlers.NewUploadChunkHandler(&services.SessionServiceMock{})
	req, err := http.NewRequest("POST", "/upload/chunk", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/upload/chunk/{session_id}", handler.UploadChunk)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Missing session_id in URL.", response["message"])
}

func TestUploadChunkHandler_InvalidChunkID(t *testing.T) {
	handler := handlers.NewUploadChunkHandler(&services.SessionServiceMock{})
	req, err := http.NewRequest("POST", "/upload/chunk/session123", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Form = map[string][]string{"chunk_id": {"invalid"}, "checksum": {"1234"}}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/upload/chunk/{session_id}", handler.UploadChunk)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Invalid chunk_id format.", response["message"])
}

func TestUploadChunkHandler_MissingChecksum(t *testing.T) {
	handler := handlers.NewUploadChunkHandler(&services.SessionServiceMock{})
	req, err := http.NewRequest("POST", "/upload/chunk/session123", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Form = map[string][]string{"chunk_id": {"1"}}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/upload/chunk/{session_id}", handler.UploadChunk)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Missing checksum.", response["message"])
}

func TestUploadChunkHandler_ReadChunkDataError(t *testing.T) {
	handler := handlers.NewUploadChunkHandler(&services.SessionServiceMock{})
	req, err := http.NewRequest("POST", "/upload/chunk/session123", strings.NewReader("chunk data"))
	if err != nil {
		t.Fatal(err)
	}
	req.Form = map[string][]string{"chunk_id": {"1"}, "checksum": {"1234"}}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/upload/chunk/{session_id}", handler.UploadChunk)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Contains(t, response["message"], "Error reading chunk data.")
}

func TestUploadChunkHandler_Timeout(t *testing.T) {
	handler := handlers.NewUploadChunkHandler(&services.SessionServiceMock{})
	req, err := http.NewRequest("POST", "/upload/chunk/session123", strings.NewReader("chunk data"))
	if err != nil {
		t.Fatal(err)
	}
	req.Form = map[string][]string{"chunk_id": {"1"}, "checksum": {"1234"}}
	req.ContentLength = int64(1024 * 1024 * 20) // Large chunk to trigger timeout

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/upload/chunk/{session_id}", handler.UploadChunk)
	router.ServeHTTP(rr, req.WithContext(context.Background()))

	assert.Equal(t, http.StatusGatewayTimeout, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Contains(t, response["message"], "Timeout processing chunk")
}

func TestUploadChunkHandler_ChecksumValidationFailed(t *testing.T) {
	mockService := &services.SessionServiceMock{
		FileService: &services.FileServiceMock{
			ValidateChecksumFunc: func(data []byte, checksum string) bool { return false },
		},
	}
	handler := handlers.NewUploadChunkHandler(mockService)
	req, err := http.NewRequest("POST", "/upload/chunk/session123", bytes.NewReader([]byte("chunk data")))
	if err != nil {
		t.Fatal(err)
	}
	req.Form = map[string][]string{"chunk_id": {"1"}, "checksum": {"1234"}}
	req.ContentLength = int64(len("chunk data"))

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/upload/chunk/{session_id}", handler.UploadChunk)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusPreconditionFailed, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Checksum validation failed.", response["message"])
}

func TestUploadChunkHandler_Success(t *testing.T) {
	mockService := &services.SessionServiceMock{
		FileService: &services.FileServiceMock{
			ValidateChecksumFunc: func(data []byte, checksum string) bool { return true },
			SaveChunkFunc:        func(sessionID string, chunkID int, data []byte) error { return nil },
		},
	}
	handler := handlers.NewUploadChunkHandler(mockService)
	req, err := http.NewRequest("POST", "/upload/chunk/session123", bytes.NewReader([]byte("chunk data")))
	if err != nil {
		t.Fatal(err)
	}
	req.Form = map[string][]string{"chunk_id": {"1"}, "checksum": {"1234"}}
	req.ContentLength = int64(len("chunk data"))

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/upload/chunk/{session_id}", handler.UploadChunk)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Chunk 1 uploaded successfully.", response["message"])
}

func TestUploadChunkHandler_ChunkAlreadyExists(t *testing.T) {
	mockService := &services.SessionServiceMock{
		FileService: &services.FileServiceMock{
			ValidateChecksumFunc: func(data []byte, checksum string) bool { return true },
			SaveChunkFunc: func(sessionID string, chunkID int, data []byte) error {
				return services.ErrChunkAlreadyExists
			},
		},
	}
	handler := handlers.NewUploadChunkHandler(mockService)
	req, err := http.NewRequest("POST", "/upload/chunk/session123", bytes.NewReader([]byte("chunk data")))
	if err != nil {
		t.Fatal(err)
	}
	req.Form = map[string][]string{"chunk_id": {"1"}, "checksum": {"1234"}}
	req.ContentLength = int64(len("chunk data"))

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/upload/chunk/{session_id}", handler.UploadChunk)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	var response map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&response)
	assert.Equal(t, "Chunk already uploaded.", response["message"])
}
