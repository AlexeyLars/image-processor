package server

import (
	"github.com/AlexeyLars/image-processor/logic-service/internal/processor"
	pb "github.com/AlexeyLars/image-processor/shared/proto"
	"log/slog"
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
