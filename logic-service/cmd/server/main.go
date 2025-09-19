package main

import (
	"github.com/AlexeyLars/image-processor/logic-service/internal/config"
	"log/slog"
	"net"
	"os"
)

func main() {
	// Load config and extract log level from it
	cfg := config.Load()
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// Configure logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	slog.Info("Starting logic service",
		"service", "logic-service",
		"grpc-addr", cfg.GRPCAddr,
		"log_level", cfg.LogLevel)

	// Create listener
	_, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		slog.Error("Failed to create listener",
			"error", err,
			"addr", cfg.GRPCAddr)
		os.Exit(1)
	}

}
