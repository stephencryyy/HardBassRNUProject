
package services

import (
  "BASProject/internal/storage"
  "errors"
  "fmt"
  "log"
  "path/filepath"
  "os"
  "strconv"
)

type SessionService struct {
  Storage     *storage.RedisClient
  FileService *FileService
}

type ISessionService interface {
  CreateSession(fileName string, fileSize int64, fileHash string) (int64, error)
  GetUploadStatus(fileHash string) (map[string]interface{}, error)
  UpdateProgress(fileHash string) error // Обновлено
  DeleteSession(fileHash string) error
  GetFileService() IFileService
}


func NewSessionService(storage *storage.RedisClient, fileService *FileService) *SessionService {
  return &SessionService{
    Storage:     storage,
    FileService: fileService,
  }
}

var ErrSessionNotFound = errors.New("session not found")

// CreateSession creates a new file upload session using the file hash provided by the client.
func (s *SessionService) CreateSession(fileName string, fileSize int64, fileHash string) (int64, error) {
  if fileName == "" || fileSize <= 0 || fileHash == "" {
    return 0, errors.New("invalid file name, file size, or file hash")
  }

  // Проверяем, существует ли сессия по хешу
  exists, err := s.Storage.SessionExists(fileHash)
  if err != nil {
    return 0, fmt.Errorf("failed to check session existence: %w", err)
  }
  if exists > 0 {
    // Если сессия существует, получаем её статус
    sessionData, err := s.Storage.GetSessionData(fileHash)
    if err != nil {
      return 0, fmt.Errorf("failed to retrieve session data: %w", err)
    }

    sessionStatus, ok := sessionData["status"].(string)
    if !ok {
      log.Printf("Invalid session status: %v", sessionData)
      return 0, errors.New("invalid session status")
    }

    switch sessionStatus {
    case "completed":
      // Удаляем текущую сессию и начинаем заново
      err = s.DeleteSession(fileHash)
      if err != nil {
        return 0, fmt.Errorf("failed to delete completed session: %w", err)
      }

    case "in_progress":
      // Возвращаем существующую информацию о чанках
      return sessionData["chunk_size"].(int64), nil
    }
  }

  // Новая сессия: определяем размер чанков
  maxChunkSize := int64(1024 * 1024 * 1024) // 1GB max chunk size
  chunkSize := s.FileService.CalculateChunkSize(fileSize, maxChunkSize)

  log.Printf("Creating session with fileHash: %s", fileHash)
  sessionData := map[string]interface{}{
    "file_name":     fileName,
    "file_size":     fileSize,
    "chunk_size":    chunkSize,
    "uploaded_size": 0,
    "status":        "in_progress",
  }
  err = s.Storage.SaveSession(fileHash, sessionData)
  if err != nil {
    log.Printf("Error saving session to Redis: %v", err)
    return 0, fmt.Errorf("failed to save session: %w", err)
  }
  log.Printf("Session %s saved successfully", fileHash)
  return chunkSize, nil
}

// UpdateProgress updates the progress of the file upload for the specified file hash.
func (s *SessionService) UpdateProgress(fileHash string) error {
  sessionData, err := s.Storage.GetSessionData(fileHash)
  if err != nil {
      log.Printf("Error retrieving session data: %v", err)
      return fmt.Errorf("failed to retrieve session data: %w", err)
  }

  fileSize, err := extractInt64(sessionData["file_size"])
  if err != nil {
    return fmt.Errorf("invalid file size in session data: %v", err)
  }

  chunkSize, err := extractInt64(sessionData["chunk_size"])
  if err != nil {
    return fmt.Errorf("invalid chunk size in session data: %v", err)
  }
  totalChunks := int((fileSize + chunkSize - 1) / chunkSize)

  // Проверяем фактическое наличие чанков на диске и вычисляем uploadedSize
  uploadedChunksCount, err := s.FileService.CountChunks(fileHash)
  if err != nil {
      log.Printf("Error counting chunks: %v", err)
      return fmt.Errorf("failed to count chunks: %w", err)
  }

  uploadedSize := int64(0)
  // Пример для UpdateProgress
  for i := 1; i <= totalChunks; i++ {
    chunkFile := fmt.Sprintf("./data/%s_%d.part", fileHash, i)
    if fi, err := os.Stat(chunkFile); err == nil {
      uploadedSize += fi.Size()
    }
  }


  sessionData["uploaded_size"] = uploadedSize
  err = s.Storage.SaveSession(fileHash, sessionData)
  if err != nil {
      log.Printf("Error saving updated session data: %v", err)
      return fmt.Errorf("failed to save updated session data: %w", err)
  }

  // Проверка завершенности загрузки
  if uploadedSize >= fileSize && uploadedChunksCount >= totalChunks {
      sessionData["status"] = "completed"
      err = s.Storage.SaveSession(fileHash, sessionData)
      if err != nil {
          log.Printf("Error saving session to Redis: %v", err)
          return fmt.Errorf("failed to save session: %w", err)
      }
      log.Printf("Session %s marked as completed", fileHash)
  } else {
      // Если не все чанки на месте, устанавливаем статус in_progress
      sessionData["status"] = "in_progress"
      err = s.Storage.SaveSession(fileHash, sessionData)
      if err != nil {
          log.Printf("Error saving session to Redis: %v", err)
          return fmt.Errorf("failed to save session: %w", err)
      }
  }

  return nil
}


// GetUploadStatus retrieves the current status of the upload for the specified file hash.
func (s *SessionService) GetUploadStatus(fileHash string) (map[string]interface{}, error) {
  // Получаем данные сессии из Redis
  exists, err := s.Storage.SessionExists(fileHash)
  if err != nil {
      return nil, fmt.Errorf("failed to check session existence: %w", err)
  }
  if exists == 0 {
      return nil, ErrSessionNotFound
  }

  sessionData, err := s.Storage.GetSessionData(fileHash)
  if err != nil {
      return nil, fmt.Errorf("failed to retrieve session data: %w", err)
  }

  // Extract and convert fileSize
  fileSize, err := extractInt64(sessionData["file_size"])
  if err != nil {
    return nil, fmt.Errorf("invalid file size in session data: %v", err)
  }

  chunkSize, err := extractInt64(sessionData["chunk_size"])
  if err != nil {
    return nil, fmt.Errorf("invalid chunk size in session data: %v", err)
  }

  totalChunks := int((fileSize + chunkSize - 1) / chunkSize)

  // Проверяем фактическое наличие чанков на диске
  uploadedChunks := []int{}
  for i := 1; i <= totalChunks; i++ {
    chunkFile := fmt.Sprintf("./data/%s_%d.part", fileHash, i)
      if _, err := os.Stat(chunkFile); err == nil {
        uploadedChunks = append(uploadedChunks, i)
    } 
  }

  // Определяем список ожидающих чанков
  pendingChunks := []int{}
  uploadedChunksMap := make(map[int]bool)
  for _, chunkID := range uploadedChunks {
      uploadedChunksMap[chunkID] = true
  }

  for i := 1; i <= totalChunks; i++ {
      if !uploadedChunksMap[i] {
          pendingChunks = append(pendingChunks, i)
      }
  }

  isComplete := len(pendingChunks) == 0
  message := "Upload is in progress."
  if isComplete {
      message = "Upload is complete."
  }

  // Обновляем uploaded_size на основе фактически загруженных чанков
  uploadedSizeInterface, ok := sessionData["uploaded_size"]
  if !ok {
      return nil, fmt.Errorf("uploaded_size not found in session data")
  }
  var uploadedSize int64
  switch v := uploadedSizeInterface.(type) {
  case int64:
      uploadedSize = v
  case float64:
      uploadedSize = int64(v)
  case string:
      uploadedSizeParsed, err := strconv.ParseInt(v, 10, 64)
      if err != nil {
          return nil, fmt.Errorf("invalid uploaded size format in session data")
      }
      uploadedSize = uploadedSizeParsed
  default:
      return nil, fmt.Errorf("unexpected type for uploaded_size in session data")
  }


  // Формируем статус загрузки
  status := map[string]interface{}{
      "file_name":       sessionData["file_name"],
      "file_size":       fileSize,
      "uploaded_size":   uploadedSize,
      "completed":       isComplete,
      "remaining_size":  fileSize - uploadedSize,
      "status":          sessionData["status"].(string),
      "uploaded_chunks": uploadedChunks,
      "pending_chunks":  pendingChunks,
      "total_chunks":    totalChunks,
      "message":         message,
  }

  return status, nil
}


// DeleteSession deletes a session and its associated chunk files.
func (s *SessionService) DeleteSession(fileHash string) error {
  // Проверяем, существует ли сессия
  exists, err := s.Storage.SessionExists(fileHash)
  if err != nil {
    return fmt.Errorf("failed to check session existence: %w", err)
  }
  if exists == 0 {
    return ErrSessionNotFound
  }
  err = s.Storage.DeleteSessionData(fileHash)
  if err != nil {
    return fmt.Errorf("failed to delete session data: %w", err)
  }

  // Удаляем файлы чанков с диска
  err = s.FileService.DeleteChunks(fileHash)
  if err != nil {
    return fmt.Errorf("failed to delete chunk files: %w", err)
  }

  return nil
}

func (s *SessionService) GetFileService() IFileService {
  return s.FileService
}

// CountChunks counts the number of chunk files for the given fileHash in the data folder.
func (fs *FileService) CountChunks(fileHash string) (int, error) {
  pattern := fmt.Sprintf("./data/%s_*.part", fileHash)
  files, err := filepath.Glob(pattern)
  if err != nil {
      return 0, fmt.Errorf("failed to list chunk files: %w", err)
  }
  return len(files), nil
}
func extractInt64(value interface{}) (int64, error) {
  switch v := value.(type) {
  case int64:
      return v, nil
  case float64:
      return int64(v), nil
  case string:
      return strconv.ParseInt(v, 10, 64)
  default:
      return 0, fmt.Errorf("unexpected type %T", v)
  }
}