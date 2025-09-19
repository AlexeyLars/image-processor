package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"log/slog"
)

type Config struct {
	GRPCAddr string `env:"GRPC_ADDR" env-default:":50051" env-description:"gRPC server address"`
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

	slog.Info("Configuration loaded successfully",
		"grpc_addr", cfg.GRPCAddr,
		"log_level", cfg.LogLevel,
		"component", "config")

	return &cfg
}
