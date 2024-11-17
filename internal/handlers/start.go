package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"BASProject/internal/services"
)

type StartHandler struct {
	SessionService services.ISessionService
}

func NewStartHandler(sessionService services.ISessionService) *StartHandler {
	return &StartHandler{
		SessionService: sessionService,
	}
}

func (h *StartHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		FileName string `json:"file_name"`
		FileSize int64  `json:"file_size"`
		FileHash string `json:"file_hash"`
	}

	// Декодируем данные из тела запроса
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		log.Printf("Ошибка при декодировании JSON: %v", err)
		return
	}

	// Проверяем корректность полученных данных
	if requestData.FileName == "" || requestData.FileSize <= 0 || requestData.FileHash == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "error",
			"error_code": 400,
			"message":    "Invalid request. Missing or incorrect parameters.",
			"details":    "File name, size, and hash are required.",
		})
		log.Println("Ошибка: недостающие или некорректные параметры")
		return
	}

	// Создаем сессию, используя полученные данные
	chunkSize, err := h.SessionService.CreateSession(requestData.FileName, requestData.FileSize, requestData.FileHash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "error",
			"error_code": 500,
			"message":    "Internal server error.",
			"details":    err.Error(),
		})
		log.Printf("Ошибка при создании сессии: %v", err)
		return
	}

	// Ответ с размером чанка
	responseData := map[string]interface{}{
		"chunk_size": chunkSize,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(responseData); err != nil {
		log.Printf("Ошибка при отправке ответа: %v", err)
	}
}
