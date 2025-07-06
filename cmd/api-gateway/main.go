package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/internal/gateway"
)

func main() {
	// Initialize logger
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)

	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	// Set log level from config
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.WithError(err).Warn("Invalid log level, using info")
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	log.Info("Starting API Gateway Service")

	// Initialize service
	service, err := gateway.NewService(cfg.APIGateway, cfg.Security, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create API Gateway service")
	}

	// Start server in goroutine
	go func() {
		if err := service.Start(); err != nil {
			log.WithError(err).Fatal("Failed to start API Gateway server")
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down API Gateway Service")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := service.Stop(shutdownCtx); err != nil {
		log.WithError(err).Error("Failed to shutdown API Gateway gracefully")
	}

	log.Info("API Gateway Service shutdown complete")
}
