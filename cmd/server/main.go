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
	"os"

	"github.com/gorilla/mux"
)

func main() {
	// Параметры командной строки для порта и пути к хранилищу
	port := flag.Int("port", 0, "Port for the server (overrides config)")
	storagePath := flag.String("storage", "", "Path to storage (overrides config)")
	flag.Parse()

	// Загрузка конфигурации
	cfgPath := "config/config.yaml"
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// После обработки флагов командной строки в сервере
	if *port != 0 {
		cfg.Server.Port = *port

		// Сохраняем обновленную конфигурацию обратно в файл
		err = config.SaveConfig(cfgPath, cfg)
		if err != nil {
			log.Fatalf("Error saving updated config: %v", err)
		}
	}

	if *storagePath != "" {
		cfg.Storage.Path = *storagePath
	}

	// Убеждаемся, что путь к хранилищу существует
	if _, err := os.Stat(cfg.Storage.Path); os.IsNotExist(err) {
		log.Printf("Storage path %s does not exist. Creating...", cfg.Storage.Path)
		if err := os.MkdirAll(cfg.Storage.Path, os.ModePerm); err != nil {
			log.Fatalf("Failed to create storage path %s: %v", cfg.Storage.Path, err)
		}
	}

	// Сохраняем обновленную конфигурацию обратно в файл
	err = config.SaveConfig(cfgPath, cfg)
	if err != nil {
		log.Fatalf("Error saving updated config: %v", err)
	}

	// Инициализация сервисов и обработчиков
	redisClient := storage.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	fileService := services.NewFileService(redisClient, cfg.Storage.Path)
	sessionService := services.NewSessionService(redisClient, fileService)
	startHandler := handlers.NewStartHandler(sessionService)
	uploadChunkHandler := handlers.NewUploadChunkHandler(sessionService)
	statusHandler := handlers.NewStatusHandler(sessionService)
	deleteHandler := handlers.NewDeleteHandler(sessionService)

	// Настройка маршрутов
	router := mux.NewRouter()
	router.HandleFunc("/upload/start", startHandler.StartSession).Methods("POST")
	router.HandleFunc("/upload/{session_id}/chunk", uploadChunkHandler.UploadChunk).Methods("POST")
	router.HandleFunc("/upload/complete/{session_id}", uploadChunkHandler.CompleteUpload).Methods("POST")
	router.HandleFunc("/upload/status/{session_id}", statusHandler.GetUploadStatus).Methods("GET")
	router.HandleFunc("/upload/{session_id}", deleteHandler.DeleteSession).Methods("DELETE")

	// Запуск сервера
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Server is running on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
