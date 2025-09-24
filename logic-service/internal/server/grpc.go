package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/AlexeyLars/image-processor/logic-service/internal/config"
	"github.com/AlexeyLars/image-processor/logic-service/internal/metrics"
	"github.com/AlexeyLars/image-processor/logic-service/internal/processor"
	pb "github.com/AlexeyLars/image-processor/shared/proto"
	"golang.org/x/sync/semaphore"
	"log/slog"
	"time"
)

type ImageProcessorService struct {
	pb.UnimplementedImageProcessorServer
	processor *processor.ImageProcessor
	logger    *slog.Logger
	config    *config.Config
	metrics   *metrics.Metrics
	memorySem *semaphore.Weighted
}

func NewImageProcessorService(logger *slog.Logger, cfg *config.Config, metrics *metrics.Metrics) *ImageProcessorService {
	// Create weighted semaphore based on config limits
	maxMemoryBytes := cfg.ImageLimits.MaxConcurrentMemoryMB * 1024 * 1024
	memorySem := semaphore.NewWeighted(maxMemoryBytes)

	logger.Info("ImageProcessorService initialized",
		"component", "grpc_server",
		"max_concurrent_memory_mb", cfg.ImageLimits.MaxConcurrentMemoryMB,
		"max_image_size_mb", cfg.ImageLimits.MaxImageSizeMB,
		"processing_timeout_sec", cfg.ImageLimits.ProcessingTimeoutSec)

	return &ImageProcessorService{
		processor: processor.NewImageProcessor(),
		logger:    logger.With("component", "grpc_server"),
		config:    cfg,
		metrics:   metrics,
		memorySem: memorySem,
	}
}

// convertFilterType converts protobuf FilterType into processor filter type
func convertFilterType(pbType pb.FilterType) processor.FilterType {
	switch pbType {
	case pb.FilterType_BLUR:
		return processor.FilterBlur
	case pb.FilterType_SHARPEN:
		return processor.FilterSharpen
	case pb.FilterType_GRAYSCALE:
		return processor.FilterGrayscale
	case pb.FilterType_SEPIA:
		return processor.FilterSepia
	case pb.FilterType_BRIGHTNESS:
		return processor.FilterBrightness
	case pb.FilterType_CONTRAST:
		return processor.FilterContrast
	default:
		return processor.FilterBlur
	}
}

func (s *ImageProcessorService) ApplyFilter(ctx context.Context, req *pb.FilterRequest) (*pb.FilterResponse, error) {
	startTime := time.Now()
	method := "apply_filter"
	s.metrics.RecordRequestStart(method)
	defer func() {
		processingDuration := time.Since(startTime).Seconds()
		s.metrics.RecordProcessingDuration(method, processingDuration)
	}()

	s.logger.Info("Processing filter request",
		"operation", method,
		"filter_type", req.FilterType.String(),
		"intensity", req.Intensity,
		"session_id", req.SessionId,
		"image_size_bytes", len(req.ImageData))

	// 1) Check image size before semaphore
	imageSize := int64(len(req.ImageData))
	maxImageSize := s.config.ImageLimits.MaxImageSizeMB * 1024 * 1024
	if imageSize > maxImageSize {
		s.logger.Warn("Image too large",
			"size_mb", imageSize/(1024*1024),
			"max_allowed_mb", s.config.ImageLimits.MaxImageSizeMB,
			"session_id", req.SessionId)

		s.metrics.RecordError(method, "image_too_large")
		s.metrics.RecordRequestEnd(method, "failed")

		return &pb.FilterResponse{
			Success: false,
			ErrorMessage: fmt.Sprintf("Image too large: %d MB, max allowed: %d MB",
				imageSize/(1024*1024), s.config.ImageLimits.MaxImageSizeMB),
		}, nil
	}

	s.metrics.RecordImageSize("input", imageSize)

	// 2) Create ctx with timeout
	processingTimeout := time.Duration(s.config.ImageLimits.ProcessingTimeoutSec) * time.Second
	processingCtx, cancel := context.WithTimeout(ctx, processingTimeout)
	defer cancel()

	// 3) Capturing semaphore with img size
	if err := s.memorySem.Acquire(processingCtx, imageSize); err != nil {
		errorType := "semaphore_timeout"
		if errors.Is(err, context.Canceled) {
			errorType = "client_disconnected"
		} else if errors.Is(err, context.DeadlineExceeded) {
			errorType = "processing_timeout"
		}

		s.logger.Warn("Failed to acquire semaphore",
			"error", err,
			"session_id", req.SessionId,
			"image_size_bytes", imageSize)

		s.metrics.RecordError(method, errorType)
		s.metrics.RecordRequestEnd(method, "failed")

		return &pb.FilterResponse{
			Success:      false,
			ErrorMessage: "System busy or request timeout, try again later",
		}, nil
	}
	defer s.memorySem.Release(imageSize)

	// 4) Process image
	img, format, err := s.processor.DecodeImage(req.ImageData)
	if err != nil {
		s.logger.Error("Failed to decode image",
			"operation", method,
			"session_id", req.SessionId,
			"error", err)

		s.metrics.RecordError(method, "decode_error")
		s.metrics.RecordRequestEnd(method, "failed")
		return &pb.FilterResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to decode image: %v", err),
		}, nil
	}

	// Check ctx before operation
	select {
	case <-processingCtx.Done():
		s.logger.Info("Processing cancelled before filter apply",
			"reason", processingCtx.Err().Error(),
			"session_id", req.SessionId)

		s.metrics.RecordError(method, "cancelled")
		s.metrics.RecordRequestEnd(method, "failed")
		return &pb.FilterResponse{
			Success:      false,
			ErrorMessage: "Processing cancelled",
		}, nil
	default: // continue
	}

	filterType := convertFilterType(req.FilterType) // protobuf filter type -> processor filter type

	processedImg := s.processor.ApplyFilter(img, filterType, float64(req.Intensity))

	resultData, err := s.processor.EncodeImage(processedImg, format)
	if err != nil {
		s.logger.Error("Failed to encode processed image",
			"operation", method,
			"session_id", req.SessionId,
			"error", err)

		s.metrics.RecordError(method, "encode_error")
		s.metrics.RecordRequestEnd(method, "failed")
		return &pb.FilterResponse{
			Success:      false,
			ErrorMessage: "Failed to encode result",
		}, nil
	}

	// Record metrics
	s.metrics.RecordImageSize("output", int64(len(resultData)))
	s.metrics.RecordRequestEnd(method, "success")
	s.metrics.RecordFilterApplied(req.FilterType.String())

	processingTime := time.Since(startTime).Milliseconds()

	s.logger.Info("Filter applied successfully",
		"operation", method,
		"session_id", req.SessionId,
		"filter_type", req.FilterType.String(),
		"processing_time_ms", processingTime,
		"input_size_bytes", len(req.ImageData),
		"output_size_bytes", len(resultData),
		"format", format)

	return &pb.FilterResponse{
		ProcessedImage:   resultData,
		Success:          true,
		ProcessingTimeMs: processingTime,
	}, nil
}

func (s *ImageProcessorService) ResizeImage(ctx context.Context, req *pb.ResizeRequest) (*pb.ResizeResponse, error) {
	startTime := time.Now()
	method := "resize_image"

	s.metrics.RecordRequestStart(method)
	defer func() {
		processingDuration := time.Since(startTime).Seconds()
		s.metrics.RecordProcessingDuration(method, processingDuration)
	}()

	s.logger.Info("Processing resize request",
		"operation", method,
		"target_width", req.Width,
		"target_height", req.Height,
		"keep_aspect_ratio", req.KeepAspectRatio,
		"session_id", req.SessionId,
		"image_size_bytes", len(req.ImageData))

	// 1) Check image size before semaphore
	imageSize := int64(len(req.ImageData))
	maxImageSize := s.config.ImageLimits.MaxImageSizeMB * 1024 * 1024

	if imageSize > maxImageSize {
		s.logger.Warn("Image too large",
			"size_mb", imageSize/(1024*1024),
			"max_allowed_mb", s.config.ImageLimits.MaxImageSizeMB,
			"session_id", req.SessionId)

		s.metrics.RecordError(method, "image_too_large")
		s.metrics.RecordRequestEnd(method, "failed")

		return &pb.ResizeResponse{
			Success: false,
			ErrorMessage: fmt.Sprintf("Image too large: %dMB, max allowed: %dMB",
				imageSize/(1024*1024), s.config.ImageLimits.MaxImageSizeMB),
		}, nil
	}

	s.metrics.RecordImageSize("input", imageSize)

	// 2) Create ctx with timeout
	processingTimeout := time.Duration(s.config.ImageLimits.ProcessingTimeoutSec) * time.Second
	processingCtx, cancel := context.WithTimeout(ctx, processingTimeout)
	defer cancel()

	// 3) Capturing semaphore
	if err := s.memorySem.Acquire(processingCtx, imageSize); err != nil {
		errorType := "semaphore_timeout"
		if err == context.Canceled {
			errorType = "client_disconnected"
		} else if err == context.DeadlineExceeded {
			errorType = "processing_timeout"
		}

		s.logger.Warn("Failed to acquire semaphore",
			"error", err,
			"session_id", req.SessionId,
			"image_size_bytes", imageSize)

		s.metrics.RecordError(method, errorType)
		s.metrics.RecordRequestEnd(method, "failed")

		return &pb.ResizeResponse{
			Success:      false,
			ErrorMessage: "System busy or request timeout, try again later",
		}, nil
	}
	defer s.memorySem.Release(imageSize)

	// 4) Process image

	// Декодируем изображение
	img, format, err := s.processor.DecodeImage(req.ImageData)
	if err != nil {
		s.logger.Error("Failed to decode image",
			"operation", method,
			"session_id", req.SessionId,
			"error", err)

		s.metrics.RecordError(method, "decode_error")
		s.metrics.RecordRequestEnd(method, "failed")
		return &pb.ResizeResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to decode image: %v", err),
		}, nil
	}

	// Check ctx before resizing
	select {
	case <-processingCtx.Done():
		s.logger.Info("Processing cancelled before resize",
			"reason", processingCtx.Err().Error(),
			"session_id", req.SessionId)

		s.metrics.RecordError(method, "cancelled")
		s.metrics.RecordRequestEnd(method, "failed")
		return &pb.ResizeResponse{
			Success:      false,
			ErrorMessage: "Processing cancelled",
		}, nil
	default: // continue
	}

	resizedImg := s.processor.ResizeImage(img, int(req.Width), int(req.Height), req.KeepAspectRatio)

	bounds := resizedImg.Bounds()
	actualWidth := int32(bounds.Dx())
	actualHeight := int32(bounds.Dy())

	resultData, err := s.processor.EncodeImage(resizedImg, format)
	if err != nil {
		s.logger.Error("Failed to encode resized image",
			"operation", method,
			"session_id", req.SessionId,
			"error", err)

		s.metrics.RecordError(method, "encode_error")
		s.metrics.RecordRequestEnd(method, "failed")
		return &pb.ResizeResponse{
			Success:      false,
			ErrorMessage: "Failed to encode result",
		}, nil
	}

	// Record metrics
	s.metrics.RecordImageSize("output", int64(len(resultData)))
	s.metrics.RecordRequestEnd(method, "success")

	processingTime := time.Since(startTime).Milliseconds()

	s.logger.Info("Resize completed successfully",
		"operation", method,
		"session_id", req.SessionId,
		"target_width", req.Width,
		"target_height", req.Height,
		"actual_width", actualWidth,
		"actual_height", actualHeight,
		"keep_aspect_ratio", req.KeepAspectRatio,
		"processing_time_ms", processingTime,
		"input_size_bytes", len(req.ImageData),
		"output_size_bytes", len(resultData),
		"format", format)

	return &pb.ResizeResponse{
		ProcessedImage:   resultData,
		Success:          true,
		ActualWidth:      actualWidth,
		ActualHeight:     actualHeight,
		ProcessingTimeMs: processingTime,
	}, nil
}

func (s *ImageProcessorService) RemoveBackground(ctx context.Context, req *pb.BackgroundRemovalRequest) (*pb.BackgroundRemovalResponse, error) {
	startTime := time.Now()
	method := "remove_background"

	s.metrics.RecordRequestStart(method)
	defer func() {
		processingDuration := time.Since(startTime).Seconds()
		s.metrics.RecordProcessingDuration(method, processingDuration)
	}()

	s.logger.Info("Processing background removal request",
		"operation", method,
		"session_id", req.SessionId,
		"image_size_bytes", len(req.ImageData))

	// 1) Check img size before semaphore
	imageSize := int64(len(req.ImageData))
	maxImageSize := s.config.ImageLimits.MaxImageSizeMB * 1024 * 1024

	if imageSize > maxImageSize {
		s.logger.Warn("Image too large",
			"size_mb", imageSize/(1024*1024),
			"max_allowed_mb", s.config.ImageLimits.MaxImageSizeMB,
			"session_id", req.SessionId)

		s.metrics.RecordError(method, "image_too_large")
		s.metrics.RecordRequestEnd(method, "failed")

		return &pb.BackgroundRemovalResponse{
			Success: false,
			ErrorMessage: fmt.Sprintf("Image too large: %dMB, max allowed: %dMB",
				imageSize/(1024*1024), s.config.ImageLimits.MaxImageSizeMB),
		}, nil
	}

	s.metrics.RecordImageSize("input", imageSize)

	// 2) Create ctx with timeout
	processingTimeout := time.Duration(s.config.ImageLimits.ProcessingTimeoutSec) * time.Second
	processingCtx, cancel := context.WithTimeout(ctx, processingTimeout)
	defer cancel()

	// 3) Capturing semaphore
	if err := s.memorySem.Acquire(processingCtx, imageSize); err != nil {
		errorType := "semaphore_timeout"
		if err == context.Canceled {
			errorType = "client_disconnected"
		} else if err == context.DeadlineExceeded {
			errorType = "processing_timeout"
		}

		s.logger.Warn("Failed to acquire semaphore",
			"error", err,
			"session_id", req.SessionId,
			"image_size_bytes", imageSize)

		s.metrics.RecordError(method, errorType)
		s.metrics.RecordRequestEnd(method, "failed")

		return &pb.BackgroundRemovalResponse{
			Success:      false,
			ErrorMessage: "System busy or request timeout, try again later",
		}, nil
	}
	defer s.memorySem.Release(imageSize)

	// 4) Process image

	img, format, err := s.processor.DecodeImage(req.ImageData)
	if err != nil {
		s.logger.Error("Failed to decode image",
			"operation", method,
			"session_id", req.SessionId,
			"error", err)

		s.metrics.RecordError(method, "decode_error")
		s.metrics.RecordRequestEnd(method, "failed")
		return &pb.BackgroundRemovalResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to decode image: %v", err),
		}, nil
	}

	// Check ctx before remove
	select {
	case <-processingCtx.Done():
		s.logger.Info("Processing cancelled before background removal",
			"reason", processingCtx.Err().Error(),
			"session_id", req.SessionId)

		s.metrics.RecordError(method, "cancelled")
		s.metrics.RecordRequestEnd(method, "failed")
		return &pb.BackgroundRemovalResponse{
			Success:      false,
			ErrorMessage: "Processing cancelled",
		}, nil
	default: // continue
	}

	processedImg := s.processor.RemoveBackground(img)

	resultData, err := s.processor.EncodeImage(processedImg, format)
	if err != nil {
		s.logger.Error("Failed to encode image after background removal",
			"operation", method,
			"session_id", req.SessionId,
			"error", err)

		s.metrics.RecordError(method, "encode_error")
		s.metrics.RecordRequestEnd(method, "failed")
		return &pb.BackgroundRemovalResponse{
			Success:      false,
			ErrorMessage: "Failed to encode result",
		}, nil
	}

	// Record metrics
	s.metrics.RecordImageSize("output", int64(len(resultData)))
	s.metrics.RecordRequestEnd(method, "success")

	processingTime := time.Since(startTime).Milliseconds()

	s.logger.Info("Background removal completed (placeholder implementation)",
		"operation", method,
		"session_id", req.SessionId,
		"processing_time_ms", processingTime,
		"input_size_bytes", len(req.ImageData),
		"output_size_bytes", len(resultData),
		"format", format,
		"note", "Using blur as placeholder for background removal")

	return &pb.BackgroundRemovalResponse{
		ProcessedImage:   resultData,
		Success:          true,
		ProcessingTimeMs: processingTime,
	}, nil
}

func (s *ImageProcessorService) GetImageInfo(ctx context.Context, req *pb.ImageInfoRequest) (*pb.ImageInfoResponse, error) {
	method := "get_image_info"

	s.logger.Info("Getting image info",
		"operation", method,
		"image_size_bytes", len(req.ImageData))

	info, err := s.processor.GetImageInfo(req.ImageData)
	if err != nil {
		s.logger.Error("Failed to get image info",
			"operation", method,
			"error", err)

		s.metrics.RecordError(method, "decode_error")
		return &pb.ImageInfoResponse{}, err
	}

	s.logger.Info("Image info retrieved successfully",
		"operation", method,
		"width", info.Width,
		"height", info.Height,
		"format", info.Format,
		"size_bytes", info.SizeBytes,
		"color_mode", info.ColorMode,
		"has_alpha", info.HasAlpha)

	return &pb.ImageInfoResponse{
		Width:     info.Width,
		Height:    info.Height,
		Format:    info.Format,
		SizeBytes: info.SizeBytes,
		ColorMode: info.ColorMode,
		HasAlpha:  info.HasAlpha,
	}, nil
}
