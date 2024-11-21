package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"BASProject/config" // Импортируем пакет config
)

func main() {
	// Пример данных для отправки
	fileFlag := flag.String("file", "example.txt", "Path to the file")
	flag.Parse()

	filePath := *fileFlag

	// Определяем флаги командной строки
	portFlag := flag.Int("port", 0, "Port for the server (overrides config)")
	storageFlag := flag.String("storage", "", "Path to storage (overrides config)")
	flag.Parse()

	// Загрузка конфигурации из файла
	cfgPath := "config/config.yaml"
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Если значение порта передано через командную строку, то используем его, иначе берем из конфига
	port := cfg.Server.Port
	if *portFlag != 0 {
		port = *portFlag
	}

	// Если путь к хранилищу передан через командную строку, то используем его, иначе берем из конфига
	storagePath := cfg.Storage.Path
	if *storageFlag != "" {
		storagePath = *storageFlag
	}

	// Используем обновленные значения порта и пути к хранилищу
	serverURL := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("Server will start at %s\n", serverURL)
	fmt.Printf("Storage path is set to %s\n", storagePath)

	// Создание сессии
	fileHash, err := CalculateFileHash(filePath)
	if err != nil {
		log.Fatalf("Error calculating file hash: %v", err)
	}

	chunkSize, err := createSession(serverURL, filePath, fileHash)
	if err != nil {
		log.Fatalf("Error creating session: %v", err)
	}
	fmt.Printf("Session ID: %s, Chunk Size: %d\n", fileHash, chunkSize)

	// Открытие файла
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	// Получение размера файла
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalf("Error getting file info: %v", err)
	}
	fileSize := fileInfo.Size()
	totalChunks := int((fileSize + int64(chunkSize) - 1) / int64(chunkSize)) // Округление вверх

	// Разделение на чанки и параллельная отправка
	buf := make([]byte, chunkSize)
	begin := time.Now()

	var wg sync.WaitGroup
	chunkChan := make(chan chunkData)

	// Запуск воркеров для отправки чанков
	numWorkers := runtime.GOMAXPROCS(0)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range chunkChan {
				err := sendChunk(serverURL, fileHash, chunk.data, chunk.chunkID)
				if err != nil {
					log.Printf("Error sending chunk %d: %v", chunk.chunkID, err)
					continue
				}
			}
		}()
	}

	// Чтение файла и отправка чанков в канал
	for chunkID := 1; chunkID <= totalChunks; chunkID++ {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			log.Fatalf("Error reading file: %v", err)
		}
		if n == 0 {
			break
		}
		chunk := make([]byte, n)
		copy(chunk, buf[:n])
		chunkChan <- chunkData{chunkID: chunkID, data: chunk}
	}
	close(chunkChan)

	// Ожидание завершения всех воркеров
	wg.Wait()

	fmt.Printf("File uploaded in %v\n", time.Since(begin))
	// Завершаем передачу
	err = completeUpload(serverURL, fileHash)
	if err != nil {
		log.Fatalf("Error completing upload: %v", err)
	}
	fmt.Println("File transmission complete.")
}

type chunkData struct {
	chunkID int
	data    []byte
}

// createSession отправляет запрос на создание сессии и получает размер чанка
func createSession(serverURL, filePath, fileHash string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Fatalf("Error getting file info: %v", err)
		return 0, fmt.Errorf("error opening file: %v", err)
	}
	fileSize := fileInfo.Size()

	// Формируем тело запроса
	requestData := map[string]interface{}{
		"file_name": filePath,
		"file_size": fileSize,
		"file_hash": fileHash, // Добавляем хэш
	}
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return 0, fmt.Errorf("failed to create request body: %v", err)
	}

	// Отправка запроса на старт сессии
	url := fmt.Sprintf("%s/upload/start", serverURL)
	resp, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return 0, fmt.Errorf("failed to create session: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("server returned non-OK status: %s", resp.Status)
	}

	// Чтение ответа
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 0, fmt.Errorf("failed to parse response: %v", err)
	}

	chunkSize := int64(result["chunk_size"].(float64))
	fmt.Printf("Chunk size: %d bytes\n", chunkSize)

	return chunkSize, nil
}

// sendChunk отправляет чанк на сервер
func sendChunk(serverURL, fileHash string, data []byte, chunkID int) error {
	url := fmt.Sprintf("%s/upload/%s/chunk", serverURL, fileHash)

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
		return fmt.Errorf("server returned non-OK status: %v", resp.Status)
	}

	log.Printf("Chunk %d sent successfully", chunkID)
	return nil
}

// completeUpload отправляет запрос на завершение загрузки
func completeUpload(serverURL, fileHash string) error {
	url := fmt.Sprintf("%s/upload/complete/%s", serverURL, fileHash)

	// Отправляем запрос на завершение сессии
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to complete upload: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-OK status: %v", resp.Status)
	}

	log.Println("Upload completed successfully.")
	return nil
}

// CalculateFileHash рассчитывает хэш файла поблочно с использованием SHA-256.
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	buffer := make([]byte, 10*1024*1024) // 10 MB за раз
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return "", err
		}
		if n == 0 {
			break
		}
		hash.Write(buffer[:n])
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
