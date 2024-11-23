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
)

func main() {
	// Define command-line flags
	fileFlag := flag.String("file", "", "Path to the file")
	portFlag := flag.Int("port", 0, "Port for the server (overrides config)")
	// storageFlag := flag.String("storage", "", "Path to storage (overrides config)")
	flag.Parse()


	filePath := *fileFlag

	if filePath == "" {
		log.Fatal("Please provide a file path using the -file flag.")
	}

	// Use command-line port if provided, else use config
	if *portFlag == 0 {
		log.Fatal("Please provide a port using the -port flag.")
	}
	port := *portFlag

	// if *storageFlag == "" {
	// 	log.Fatal("Please provide a storage path using the -storage flag.")
	// }


	// Build server URL
	serverURL := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("Server will start at %s\n", serverURL)
	// fmt.Printf("Storage path is set to %s\n", *storageFlag)

	// Calculate file hash
	fileHash, err := CalculateFileHash(filePath)
	if err != nil {
		log.Fatalf("Error calculating file hash: %v", err)
	}

	// Create session and get chunk size
	chunkSize, err := createSession(serverURL, filePath, fileHash)
	if err != nil {
		log.Fatalf("Error creating session: %v", err)
	}
	fmt.Printf("Session ID: %s, Chunk Size: %d\n", fileHash, chunkSize)

	// Generate short session ID
	shortSessionID := getShortSessionID(fileHash)

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	// Get file size and calculate total chunks
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalf("Error getting file info: %v", err)
	}
	fileSize := fileInfo.Size()
	totalChunks := int((fileSize + int64(chunkSize) - 1) / int64(chunkSize)) // Round up

	// Log the start of the upload
	log.Printf("session %s: Uploading file %s in %d chunks", shortSessionID, fileInfo.Name(), totalChunks)

	// Prepare for chunked upload
	buf := make([]byte, chunkSize)
	begin := time.Now()

	var wg sync.WaitGroup
	chunkChan := make(chan chunkData)

	// Launch workers to send chunks
	numWorkers := runtime.GOMAXPROCS(0)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range chunkChan {
				// Log before sending each chunk
				log.Printf("session %s: Sending chunk %d/%d", shortSessionID, chunk.chunkID, totalChunks)

				err := sendChunk(serverURL, fileHash, chunk.data, chunk.chunkID, totalChunks, shortSessionID)
				if err != nil {
					log.Printf("Error sending chunk %d: %v", chunk.chunkID, err)
					continue
				}
			}
		}()
	}

	// Read the file and send chunks to the channel
	chunkID := 1
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			log.Fatalf("Error reading file: %v", err)
		}
		if n == 0 {
			break
		}
		// Make a copy of the buffer to avoid data races
		chunkDataCopy := make([]byte, n)
		copy(chunkDataCopy, buf[:n])
		chunkChan <- chunkData{chunkID: chunkID, data: chunkDataCopy}
		chunkID++
	}
	close(chunkChan)

	// Wait for all workers to finish
	wg.Wait()

	// Log the total upload time
	log.Printf("File uploaded in %v", time.Since(begin))

	// Complete the upload
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

// createSession sends a request to create a session and receives the chunk size
func createSession(serverURL, filePath, fileHash string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("error getting file info: %v", err)
	}
	fileSize := fileInfo.Size()

	// Form the request body
	requestData := map[string]interface{}{
		"file_name": fileInfo.Name(), // Use just the file name
		"file_size": fileSize,
		"file_hash": fileHash,
	}
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return 0, fmt.Errorf("failed to create request body: %v", err)
	}

	// Send the request to start a session
	url := fmt.Sprintf("%s/upload/start", serverURL)
	resp, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return 0, fmt.Errorf("failed to create session: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for error details
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("server returned non-OK status: %s, response: %s", resp.Status, string(bodyBytes))
	}

	// Read the response
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 0, fmt.Errorf("failed to parse response: %v", err)
	}

	chunkSize := int64(result["chunk_size"].(float64))
	fmt.Printf("Chunk size: %d bytes\n", chunkSize)

	return chunkSize, nil
}

// sendChunk sends a chunk to the server
func sendChunk(serverURL, fileHash string, data []byte, chunkID int, totalChunks int, shortSessionID string) error {
	url := fmt.Sprintf("%s/upload/%s/chunk", serverURL, fileHash)

	// Calculate SHA-256 for the chunk data
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	// Create a multipart request
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("chunk_id", strconv.Itoa(chunkID))
	writer.WriteField("checksum", checksum)

	// Add chunk data
	part, err := writer.CreateFormFile("chunk_data", "chunk")
	if err != nil {
		return fmt.Errorf("failed to create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("failed to write chunk data: %v", err)
	}
	writer.Close()

	// Send the request
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
		// Read response body for error details
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned non-OK status: %s, response: %s", resp.Status, string(bodyBytes))
	}

	// Log after successful chunk upload
	log.Printf("session %s: Chunk %d/%d sent successfully",shortSessionID , chunkID, totalChunks)
	return nil
}

// completeUpload sends a request to complete the upload
func completeUpload(serverURL, fileHash string) error {
	url := fmt.Sprintf("%s/upload/complete/%s", serverURL, fileHash)

	// Send the request to complete the session
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to complete upload: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for error details
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned non-OK status: %v, response: %s", resp.Status, string(bodyBytes))
	}

	log.Println("Upload completed successfully.")
	return nil
}

// CalculateFileHash calculates the file hash in blocks using SHA-256
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	buffer := make([]byte, 10*1024*1024) // 10 MB at a time
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


// getShortSessionID returns the first 6 characters of the session ID
func getShortSessionID(sessionID string) string {
	if len(sessionID) > 6 {
		return sessionID[:6]
	}
	return sessionID
}
