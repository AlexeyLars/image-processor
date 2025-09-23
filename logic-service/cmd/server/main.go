package main

import (
	"context"
	"github.com/AlexeyLars/image-processor/logic-service/internal/config"
	"github.com/AlexeyLars/image-processor/logic-service/internal/metrics"
	"github.com/AlexeyLars/image-processor/logic-service/internal/server"
	pb "github.com/AlexeyLars/image-processor/shared/proto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
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

	// Create metrics
	metricsInstance := metrics.NewMetrics()

	slog.Info("Starting logic service",
		"service", "logic-service",
		"grpc-addr", cfg.GRPCAddr,
		"log_level", cfg.LogLevel)

	// Start metrics endpoint in goroutine
	go startMetricsServer(cfg.MetricsAddr)

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
	imageService := server.NewImageProcessorService(logger, cfg, metricsInstance)
	pb.RegisterImageProcessorServer(s, imageService)

	// Enable reflection for comfortable debug
	reflection.Register(s)

	// Channel for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start gRPC server in separate goroutine
	go func() {
		slog.Info("Logic service ready to accept connections",
			"service", "logic-service",
			"grpc_addr", cfg.GRPCAddr)

		if err := s.Serve(lis); err != nil {
			slog.Error("Failed to serve gRPC",
				"error", err,
				"service", "logic-service")
			os.Exit(1)
		}
	}()

	// Wait for exit signal
	<-quit

	// Graceful shutdown with timout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Graceful stop gRPC server
	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("gRPC server stopped gracefully",
			"service", "logic-service")
	case <-shutdownCtx.Done():
		slog.Warn("Graceful shutdown timeout exceeded, forcing stop",
			"service", "logic-service")
		s.Stop()
	}

	slog.Info("Logic service stopped",
		"service", "logic-service")
}

func startMetricsServer(addr string) {
	http.Handle("/metrics", promhttp.Handler())

	slog.Info("Starting metrics server",
		"addr", addr,
		"endpoint", "/metrics")

	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("Failed to start metrics server",
			"error", err,
			"addr", addr)
	}
}
