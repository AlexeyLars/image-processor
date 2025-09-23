package server

import (
	"context"
	"github.com/AlexeyLars/image-processor/logic-service/internal/processor"
	pb "github.com/AlexeyLars/image-processor/shared/proto"
	"log/slog"
	"time"
)

type ImageProcessorService struct {
	pb.UnimplementedImageProcessorServer
	processor *processor.ImageProcessor
	logger    *slog.Logger
}

func NewImageProcessorService(logger *slog.Logger) *ImageProcessorService {
	return &ImageProcessorService{
		processor: processor.NewImageProcessor(),
		logger:    logger.With("component", "grpc_server"),
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

	s.logger.Info("Processing filter request",
		"operation", "apply_filter",
		"filter_type", req.FilterType.String(),
		"intensity", req.Intensity,
		"session_id", req.SessionId,
		"image_size_bytes", len(req.ImageData))

	img, format, err := s.processor.DecodeImage(req.ImageData)
	if err != nil {
		s.logger.Error("Failed to decode image",
			"operation", "apply_filter",
			"session_id", req.SessionId,
			"error", err)
		return &pb.FilterResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	filterType := convertFilterType(req.FilterType)

	processedImg := s.processor.ApplyFilter(img, filterType, float64(req.Intensity))

	resultData, err := s.processor.EncodeImage(processedImg, format)
	if err != nil {
		s.logger.Error("Failed to encode processed image",
			"operation", "apply_filter",
			"session_id", req.SessionId,
			"error", err)
		return &pb.FilterResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	processingTime := time.Since(startTime).Milliseconds()

	s.logger.Info("Filter applied successfully",
		"operation", "apply_filter",
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

	s.logger.Info("Processing resize request",
		"operation", "resize_image",
		"target_width", req.Width,
		"target_height", req.Height,
		"keep_aspect_ratio", req.KeepAspectRatio,
		"session_id", req.SessionId,
		"image_size_bytes", len(req.ImageData))

	img, format, err := s.processor.DecodeImage(req.ImageData)
	if err != nil {
		s.logger.Error("Failed to decode image",
			"operation", "resize_image",
			"session_id", req.SessionId,
			"error", err)
		return &pb.ResizeResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	resizedImg := s.processor.ResizeImage(img, int(req.Width), int(req.Height), req.KeepAspectRatio)

	bounds := resizedImg.Bounds()
	actualWidth := int32(bounds.Dx())
	actualHeight := int32(bounds.Dy())

	resultData, err := s.processor.EncodeImage(resizedImg, format)
	if err != nil {
		s.logger.Error("Failed to encode resized image",
			"operation", "resize_image",
			"session_id", req.SessionId,
			"error", err)
		return &pb.ResizeResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	processingTime := time.Since(startTime).Milliseconds()

	s.logger.Info("Resize completed successfully",
		"operation", "resize_image",
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

	s.logger.Info("Processing background remove request",
		"operation", "remove_background",
		"session_id", req.SessionId,
		"image_size_bytes", len(req.ImageData))

	img, format, err := s.processor.DecodeImage(req.ImageData)
	if err != nil {
		s.logger.Error("Failed to decode image",
			"operation", "remove_background",
			"session_id", req.SessionId,
			"error", err)
		return &pb.BackgroundRemovalResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	processedImg := s.processor.RemoveBackground(img)

	resultData, err := s.processor.EncodeImage(processedImg, format)
	if err != nil {
		s.logger.Error("Failed to encode image after background remove",
			"operation", "remove_background",
			"session_id", req.SessionId,
			"error", err)
		return &pb.BackgroundRemovalResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	processingTime := time.Since(startTime).Milliseconds()

	s.logger.Info("Background remove completed",
		"operation", "remove_background",
		"session_id", req.SessionId,
		"processing_time_ms", processingTime,
		"input_size_bytes", len(req.ImageData),
		"output_size_bytes", len(resultData),
		"format", format)

	return &pb.BackgroundRemovalResponse{
		ProcessedImage:   resultData,
		Success:          true,
		ProcessingTimeMs: processingTime,
	}, nil
}

func (s *ImageProcessorService) GetImageInfo(ctx context.Context, req *pb.ImageInfoRequest) (*pb.ImageInfoResponse, error) {
	s.logger.Info("Getting image info",
		"operation", "get_image_info",
		"image_size_bytes", len(req.ImageData))

	info, err := s.processor.GetImageInfo(req.ImageData)
	if err != nil {
		s.logger.Error("Failed to get image info",
			"operation", "get_image_info",
			"error", err)
		return &pb.ImageInfoResponse{}, err
	}

	s.logger.Info("Image info retrieved successfully",
		"operation", "get_image_info",
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
