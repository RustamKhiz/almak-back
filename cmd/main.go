package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"almak-back/internal/config"
	"almak-back/internal/database"
	"almak-back/internal/routes"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("ошибка конфигурации: %v", err)
	}

	if err = database.Connect(cfg); err != nil {
		log.Fatalf("ошибка подключения к БД: %v", err)
	}

	router := routes.SetupRouter(cfg)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		log.Printf("сервер запущен на порту %s", cfg.Port)
		if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ошибка запуска сервера: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("получен сигнал остановки, завершаем работу сервера")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = server.Shutdown(ctx); err != nil {
		log.Fatalf("ошибка при graceful shutdown: %v", err)
	}

	log.Println("сервер остановлен")
}
