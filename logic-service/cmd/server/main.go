package main

import (
	"github.com/AlexeyLars/image-processor/logic-service/internal/config"
	"github.com/AlexeyLars/image-processor/logic-service/internal/server"
	pb "github.com/AlexeyLars/image-processor/shared/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		slog.Error("Failed to create listener",
			"error", err,
			"addr", cfg.GRPCAddr)
		os.Exit(1)
	}

	// Create gRPC server
	s := grpc.NewServer()

	// Create and register ImageProcessor service
	imageService := server.NewImageProcessorService(logger)
	pb.RegisterImageProcessorServer(s, imageService)

	// Enable reflection for comfortable debug
	reflection.Register(s)

	slog.Info("Logic service ready to accept connections",
		"service", "logic-service",
		"grpc_addr", cfg.GRPCAddr)

	if err := s.Serve(lis); err != nil {
		slog.Error("Failed to serve gRPC",
			"error", err,
			"service", "logic-service")
		os.Exit(1)
	}
}
