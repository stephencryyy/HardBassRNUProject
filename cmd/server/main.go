package main

import (
	"BASProject/config"
	"BASProject/internal/handlers"
	"BASProject/internal/services"
	"BASProject/internal/storage"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	// Чтение аргументов командной строки
	port := flag.Int("port", 0, "Port for the server (overrides config)")
	storagePath := flag.String("storage", "", "Path to storage (overrides config)")
	flag.Parse()

	// Загрузка конфигурации
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Переопределение порта и пути хранилища при необходимости
	if *port != 0 {
		cfg.Server.Port = *port
	}
	if *storagePath != "" {
		cfg.Storage.Path = *storagePath
	}

	// Инициализация Redis
	redisClient := storage.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)

	// Инициализация FileService с Redis клиентом и путем хранения
	fileService := services.NewFileService(redisClient, cfg.Storage.Path)

	// Инициализация SessionService с FileService
	sessionService := services.NewSessionService(redisClient, fileService)

	// Инициализация обработчиков
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
