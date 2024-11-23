package test

import (
	"BASProject/internal/handlers"
	"BASProject/internal/services"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"mime/multipart"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)


func TestUploadChunkHandler_InvalidChunkID(t *testing.T) {
    handler := handlers.NewUploadChunkHandler(&services.SessionServiceMock{})

    // Создаём multipart/form-data запрос
    var b bytes.Buffer
    w := multipart.NewWriter(&b)
    w.WriteField("chunk_id", "invalid") // Некорректный chunk_id
    w.WriteField("checksum", "1234")
    w.CreateFormFile("chunk_data", "chunk") // Добавляем пустой файл
    w.Close()

    req, err := http.NewRequest("POST", "/upload/session123/chunk", &b)
    if err != nil {
        t.Fatal(err)
    }
    req.Header.Set("Content-Type", w.FormDataContentType())

    rr := httptest.NewRecorder()
    router := mux.NewRouter()
    router.HandleFunc("/upload/{session_id}/chunk", handler.UploadChunk)
    router.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusBadRequest, rr.Code)
    var response map[string]interface{}
    json.NewDecoder(rr.Body).Decode(&response)
    assert.Equal(t, "Invalid chunk_id format.", response["message"])
}


func TestUploadChunkHandler_MissingChecksum(t *testing.T) {
    handler := handlers.NewUploadChunkHandler(&services.SessionServiceMock{})

    var b bytes.Buffer
    w := multipart.NewWriter(&b)
    w.WriteField("chunk_id", "1")
    // Пропускаем поле checksum
    w.CreateFormFile("chunk_data", "chunk")
    w.Close()

    req, err := http.NewRequest("POST", "/upload/session123/chunk", &b)
    if err != nil {
        t.Fatal(err)
    }
    req.Header.Set("Content-Type", w.FormDataContentType())

    rr := httptest.NewRecorder()
    router := mux.NewRouter()
    router.HandleFunc("/upload/{session_id}/chunk", handler.UploadChunk)
    router.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusBadRequest, rr.Code)
    var response map[string]interface{}
    json.NewDecoder(rr.Body).Decode(&response)
    assert.Equal(t, "Missing checksum.", response["message"])
}


func TestUploadChunkHandler_ReadChunkDataError(t *testing.T) {
    handler := handlers.NewUploadChunkHandler(&services.SessionServiceMock{})

    // Создаём запрос без поля chunk_data
    var b bytes.Buffer
    w := multipart.NewWriter(&b)
    w.WriteField("chunk_id", "1")
    w.WriteField("checksum", "1234")
    w.Close()

    req, err := http.NewRequest("POST", "/upload/session123/chunk", &b)
    if err != nil {
        t.Fatal(err)
    }
    req.Header.Set("Content-Type", w.FormDataContentType())

    rr := httptest.NewRecorder()
    router := mux.NewRouter()
    router.HandleFunc("/upload/{session_id}/chunk", handler.UploadChunk)
    router.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusBadRequest, rr.Code)
    var response map[string]interface{}
    json.NewDecoder(rr.Body).Decode(&response)
    assert.Contains(t, response["message"], "Error reading chunk data.")
}


func TestUploadChunkHandler_ChecksumValidationFailed(t *testing.T) {
    mockService := &services.SessionServiceMock{
        FileService: &services.FileServiceMock{
            ValidateChecksumFunc: func(data []byte, checksum string) bool { return false },
        },
    }
    handler := handlers.NewUploadChunkHandler(mockService)

    var b bytes.Buffer
    w := multipart.NewWriter(&b)
    w.WriteField("chunk_id", "1")
    w.WriteField("checksum", "1234")
    fw, err := w.CreateFormFile("chunk_data", "chunk")
    if err != nil {
        t.Fatal(err)
    }
    fw.Write([]byte("chunk data"))
    w.Close()

    req, err := http.NewRequest("POST", "/upload/session123/chunk", &b)
    if err != nil {
        t.Fatal(err)
    }
    req.Header.Set("Content-Type", w.FormDataContentType())

    rr := httptest.NewRecorder()
    router := mux.NewRouter()
    router.HandleFunc("/upload/{session_id}/chunk", handler.UploadChunk)
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

    var b bytes.Buffer
    w := multipart.NewWriter(&b)
    w.WriteField("chunk_id", "1")
    w.WriteField("checksum", "1234")
    fw, err := w.CreateFormFile("chunk_data", "chunk")
    if err != nil {
        t.Fatal(err)
    }
    fw.Write([]byte("chunk data"))
    w.Close()

    req, err := http.NewRequest("POST", "/upload/session123/chunk", &b)
    if err != nil {
        t.Fatal(err)
    }
    req.Header.Set("Content-Type", w.FormDataContentType())

    rr := httptest.NewRecorder()
    router := mux.NewRouter()
    router.HandleFunc("/upload/{session_id}/chunk", handler.UploadChunk)
    router.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusOK, rr.Code)
    var response map[string]interface{}
    json.NewDecoder(rr.Body).Decode(&response)
    assert.Equal(t, "success", response["status"])
    assert.Equal(t, "Chunk 1 uploaded successfully.", response["message"])
}


