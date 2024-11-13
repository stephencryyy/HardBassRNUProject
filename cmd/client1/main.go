package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	// Пример данных для отправки
	filePath := "example.txt"

	// Создание сессии
	sessionID, chunkSize, err := createSession(filePath)
	if err != nil {
		log.Fatalf("Error creating session: %v", err)
	}
	fmt.Printf("Session ID: %s, Chunk Size: %d\n", sessionID, chunkSize)

	// Открытие файла
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	// Разделение на чанки
	buf := make([]byte, chunkSize)
	chunkID := 1
	begin := time.Now()
	for {
		n, err := file.Read(buf)
		if err != nil && err.Error() != "EOF" {
			log.Fatalf("Error reading file: %v", err)
		}
		if n == 0 {
			break
		}

		// Отправка чанка с указанием chunk_id и контрольной суммой
		err = sendChunk(sessionID, buf[:n], chunkID)
		if err != nil {
			log.Fatalf("Error sending chunk: %v", err)
		}
		chunkID++
	}
	fmt.Printf("Chunk %d sent in %v\n", chunkID, time.Since(begin))

	// Завершаем передачу
	err = completeUpload(sessionID)
	if err != nil {
		log.Fatalf("Error completing upload: %v", err)
	}
	fmt.Println("File transmission complete.")
}

// createSession отправляет запрос на создание сессии и получает GUID
func createSession(filePath string) (string, int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	fileSize := fileInfo.Size()

	// Добавим вывод для проверки
	fmt.Printf("File size: %d bytes\n", fileSize)

	if fileSize <= 0 {
		log.Fatalf("File size is zero or negative, cannot proceed")
	}

	// Формируем тело запроса
	requestData := map[string]interface{}{
		"file_name": filePath,
		"file_size": fileSize,
	}
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request body: %v", err)
	}

	// Отправка запроса на старт сессии
	resp, err := http.Post("http://localhost:6382/upload/start", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create session: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("server returned non-OK status: %s", resp.Status)
	}

	// Чтение ответа
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse response: %v", err)
	}

	sessionID := result["session_id"].(string)
	chunkSize := int64(result["chunk_size"].(float64))

	return sessionID, chunkSize, nil
}

// sendChunk отправляет чанк на сервер
func sendChunk(sessionID string, data []byte, chunkID int) error {
	url := fmt.Sprintf("http://localhost:6382/upload/%s/chunk", sessionID)

	// Вычисляем SHA-256 для данных чанка
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	// Создаем multipart-запрос
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("chunk_id", strconv.Itoa(chunkID))
	writer.WriteField("checksum", checksum)

	// Добавляем данные чанка
	part, err := writer.CreateFormFile("chunk_data", "chunk")
	if err != nil {
		return fmt.Errorf("failed to create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("failed to write chunk data: %v", err)
	}
	writer.Close()

	// Отправляем запрос
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send chunk: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-OK status: %s", resp.Status)
	}

	log.Printf("Chunk %d sent successfully", chunkID)
	return nil
}

// completeUpload отправляет запрос для завершения загрузки
func completeUpload(sessionID string) error {
	url := fmt.Sprintf("http://localhost:6382/upload/complete/%s", sessionID)

	// Отправляем запрос на завершение сессии
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to complete upload: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-OK status: %s", resp.Status)
	}

	fmt.Printf("Upload session %s completed successfully.\n", sessionID)
	return nil
}
