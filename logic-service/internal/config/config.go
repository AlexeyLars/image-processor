package config

import (
	shared "github.com/AlexeyLars/image-processor/shared/configs"
	"github.com/ilyakaznacheev/cleanenv"
	"log/slog"
)

type Config struct {
	// gRPC server settings
	GRPCAddr string `env:"GRPC_ADDR" env-default:":50051" env-description:"gRPC server address"`

	// Metrics settings
	MetricsAddr string `env:"METRICS_ADDR" env-default:":9090" env-description:"Prometheus metrics endpoint address"`

	// Image processing limits
	ImageLimits *shared.ImageProcessingLimits

	// Log settings
	LogLevel string `env:"LOG_LEVEL" env-default:"info" env-description:"log level: debug, info, warn, error"`
}

func Load() *Config {
	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("Failed to load configuration",
			"error", err,
			"component", "config")
		panic("Configuration loading failed")
	}

	cfg.ImageLimits = shared.GetImageProcessingLimits()

	slog.Info("Logic service configuration loaded successfully",
		"grpc_addr", cfg.GRPCAddr,
		"log_level", cfg.LogLevel,
		"metrics_addr", cfg.MetricsAddr,
		"max_image_size_mb", cfg.ImageLimits.MaxImageSizeMB,
		"max_concurrent_memory_mb", cfg.ImageLimits.MaxConcurrentMemoryMB,
		"component", "config")

	return &cfg
}
