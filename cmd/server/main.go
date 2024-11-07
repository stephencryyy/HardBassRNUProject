package main

import (
	"BASProject/internal/handlers"
	"fmt"
	"log"
	"net/http"

	"BASProject/config"
	"BASProject/internal/services"
	"BASProject/internal/storage"

	"github.com/gorilla/mux"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Инициализация Redis
	redisClient := storage.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)

	// Инициализация сервисов и обработчиков
	sessionService := services.NewSessionService(redisClient)
	startHandler := handlers.NewStartHandler(sessionService)
	uploadChunkHandler := handlers.NewUploadChunkHandler(sessionService)
	// Настройка маршрутов
	router := mux.NewRouter()
	router.HandleFunc("/upload/start", startHandler.StartSession).Methods("POST")
	router.HandleFunc("/upload/{session_id}/chunk", uploadChunkHandler.UploadChunk).Methods("POST")
	// Запуск сервера
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Server is running on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
