package services
import "errors"

// FileServiceMock — структура для мокирования IFileService в тестах.

type FileServiceMock struct {
    ValidateChecksumFunc func(data []byte, checksum string) bool
    SaveChunkFunc        func(sessionID string, chunkID int, data []byte) error
    AssembleChunksFunc   func(sessionID string, outputFilePath string) error
    DeleteChunksFunc func(sessionID string) error
}

// Реализация методов интерфейса IFileService
func (m *FileServiceMock) FileExists(fileName string) bool {
    return true
}
func (m *FileServiceMock) DeleteChunks(sessionID string) error {
	if m.DeleteChunksFunc != nil {
		return m.DeleteChunksFunc(sessionID)
	}
	return nil
}

func (m *FileServiceMock) CalculateChunkSize(fileSize, MaxChunkSize int64) int64 {
    return 1024
}

func (m *FileServiceMock) SaveChunk(sessionID string, chunkID int, data []byte) error {
    if m.SaveChunkFunc != nil {
        return m.SaveChunkFunc(sessionID, chunkID, data)
    }
    return nil
}

func (m *FileServiceMock) GetNextChunkID(sessionID string) (int, error) {
    return 1, nil
}

func (m *FileServiceMock) ValidateChecksum(data []byte, checksum string) bool {
    if m.ValidateChecksumFunc != nil {
        return m.ValidateChecksumFunc(data, checksum)
    }
    return true
}

func (m *FileServiceMock) CalculateChecksum(data []byte) string {
    return "mockedchecksum"
}

// Реализация AssembleChunks
func (m *FileServiceMock) AssembleChunks(sessionID string, outputFilePath string) error {
    if m.AssembleChunksFunc != nil {
        return m.AssembleChunksFunc(sessionID, outputFilePath)
    }
    return nil
}


// SessionServiceMock — структура для мокирования ISessionService в тестах.
type SessionServiceMock struct {
    CreateSessionFunc     func(fileName string, fileSize int64) (string, int64, error)
    GetUploadStatusFunc   func(sessionID string) (map[string]interface{}, error)
    UpdateProgressFunc    func(sessionID string, uploadedSize int64) error
    FileService           IFileService
    DeleteSessionFunc     func(sessionID string) error
}

// Реализация метода CreateSession
func (m *SessionServiceMock) CreateSession(fileName string, fileSize int64) (string, int64, error) {
    if m.CreateSessionFunc != nil {
        return m.CreateSessionFunc(fileName, fileSize)
    }
    return "", 0, errors.New("CreateSessionFunc not implemented")
}

func (m *SessionServiceMock) GetFileService() IFileService {
    return m.FileService
}


// Реализация метода GetUploadStatus
func (m *SessionServiceMock) GetUploadStatus(sessionID string) (map[string]interface{}, error) {
    if m.GetUploadStatusFunc != nil {
        return m.GetUploadStatusFunc(sessionID)
    }
    return nil, errors.New("GetUploadStatusFunc not implemented")
}

// Реализация метода UpdateProgress
func (m *SessionServiceMock) UpdateProgress(sessionID string, uploadedSize int64) error {
    if m.UpdateProgressFunc != nil {
        return m.UpdateProgressFunc(sessionID, uploadedSize)
    }
    return errors.New("UpdateProgressFunc not implemented")
}

func (m *SessionServiceMock) DeleteSession(sessionID string) error {
	if m.DeleteSessionFunc != nil {
		return m.DeleteSessionFunc(sessionID)
	}
	return nil
}