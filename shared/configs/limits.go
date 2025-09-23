package configs

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"log/slog"
)

// ImageProcessingLimits defines limits for image processing across all services
type ImageProcessingLimits struct {
	// Single image limits
	MaxImageSizeMB int64 `env:"MAX_IMAGE_SIZE_MB" env-default:"50"`
	MaxImageWidth  int   `env:"MAX_IMAGE_WIDTH" env-default:"8192"`
	MaxImageHeight int   `env:"MAX_IMAGE_HEIGHT" env-default:"8192"`

	// Concurrency limits
	MaxConcurrentMemoryMB int64 `env:"MAX_CONCURRENT_MEMORY_MB" env-default:"200"`
	MaxConcurrentJobs     int   `env:"MAX_CONCURRENT_JOBS" env-default:"10"`

	// Processing timeouts
	ProcessingTimeoutSec int `env:"PROCESSING_TIMEOUT_SEC" env-default:"30"`

	// Rate limiting
	MaxRequestsPerMinute int `env:"MAX_REQUESTS_PER_MINUTE" env-default:"100"`

	// Supported formats
	SupportedFormats []string `env:"SUPPORTED_FORMATS" env-default:"jpg,jpeg,png,gif,webp" env-separator:","`
}

// GetImageProcessingLimits returns the global limits
func GetImageProcessingLimits() *ImageProcessingLimits {
	var limits ImageProcessingLimits

	// Load from env or use defaults
	if err := cleanenv.ReadEnv(&limits); err != nil {
		// Log warning but use defaults
		slog.Warn("Failed to load image processing limits from env, using defaults",
			"error", err)
	}

	// Validation
	if err := limits.Validate(); err != nil {
		slog.Error("Invalid image processing limits configuration",
			"error", err)
		panic("Invalid configuration")
	}

	slog.Info("Image processing limits loaded",
		"max_image_size_mb", limits.MaxImageSizeMB,
		"max_concurrent_memory_mb", limits.MaxConcurrentMemoryMB,
		"max_concurrent_jobs", limits.MaxConcurrentJobs,
		"processing_timeout_sec", limits.ProcessingTimeoutSec)

	return &limits
}

// Validate checks if the limits configuration is valid
func (l *ImageProcessingLimits) Validate() error {
	if l.MaxImageSizeMB <= 0 {
		return fmt.Errorf("MaxImageSizeMB must be positive, got %d", l.MaxImageSizeMB)
	}

	if l.MaxConcurrentMemoryMB < l.MaxImageSizeMB {
		return fmt.Errorf("MaxConcurrentMemoryMB (%d) must be >= MaxImageSizeMB (%d)",
			l.MaxConcurrentMemoryMB, l.MaxImageSizeMB)
	}

	if l.MaxImageWidth <= 0 || l.MaxImageHeight <= 0 {
		return fmt.Errorf("image dimensions must be positive")
	}

	if len(l.SupportedFormats) == 0 {
		return fmt.Errorf("at least one supported format must be specified")
	}

	if l.ProcessingTimeoutSec <= 0 {
		return fmt.Errorf("ProcessingTimeoutSec must be positive")
	}

	return nil
}

// ToJSON returns limits as JSON for API responses
func (l *ImageProcessingLimits) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"maxImageSizeMB":       l.MaxImageSizeMB,
		"maxImageWidth":        l.MaxImageWidth,
		"maxImageHeight":       l.MaxImageHeight,
		"processingTimeoutSec": l.ProcessingTimeoutSec,
		"supportedFormats":     l.SupportedFormats,
		"maxRequestsPerMinute": l.MaxRequestsPerMinute,
	}
}
