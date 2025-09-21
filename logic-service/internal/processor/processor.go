package processor

import (
	"bytes"
	"fmt"
	"github.com/disintegration/imaging"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

type ImageProcessor struct {
	// Clients and etc
}

func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{}
}

// DecodeImage decodes image from bytes
func (p *ImageProcessor) DecodeImage(data []byte) (image.Image, string, error) {
	reader := bytes.NewReader(data)
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}
	return img, format, nil
}

// EncodeImage encodes image to bytes
func (p *ImageProcessor) EncodeImage(img image.Image, format string) ([]byte, error) {
	var buf bytes.Buffer

	switch format {
	case "jpeg", "jpg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, fmt.Errorf("failed to encode JPEG: %w", err)
		}
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("failed to encode PNG: %w", err)
		}
	default:
		// Save to PNG by default
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("failed to encode PNG: %w", err)
		}
	}
	return buf.Bytes(), nil
}

// Apply filter to image
func (p *ImageProcessor) ApplyFilter(img image.Image, filterType FilterType, intensity float64) image.Image {
	switch filterType {
	case FilterBlur:
		return imaging.Blur(img, intensity*5) // some scaling to better processing
	case FilterSharpen:
		return imaging.Sharpen(img, intensity*2) // some scaling to better processing
	case FilterGrayscale:
		return imaging.Grayscale(img)
	case FilterSepia:
		return p.applySepia(img, intensity)
	case FilterBrightness:
		// Convert intensity (-1.0 до 1.0) to a percent (-100 до 100)
		percent := intensity * 100
		return imaging.AdjustBrightness(img, percent)
	case FilterContrast:
		percent := intensity * 100
		return imaging.AdjustContrast(img, percent)
	default:
		return img
	}
}

func (p *ImageProcessor) ResizeImage(img image.Image, width, height int, keepAspectRatio bool) image.Image {
	if keepAspectRatio {
		return imaging.Fit(img, width, height, imaging.Lanczos)
	}
	return imaging.Resize(img, width, height, imaging.Lanczos)
}

func (p *ImageProcessor) RemoveBackground(img image.Image) image.Image {
	// TODO: background remove
	return img
}

func (p *ImageProcessor) GetImageInfo(data []byte) (*ImageInfo, error) {
	img, format, err := p.DecodeImage(data)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()

	hasAlpha := false
	colorMode := "RGB"

	switch img.ColorModel() {
	case color.RGBAModel, color.RGBA64Model, color.NRGBAModel, color.NRGBA64Model:
		hasAlpha = true
		colorMode = "RGBA"
	case color.GrayModel, color.Gray16Model:
		colorMode = "Grayscale"
	}

	return &ImageInfo{
		Width:     int32(bounds.Dx()),
		Height:    int32(bounds.Dy()),
		Format:    format,
		SizeBytes: int64(len(data)),
		ColorMode: colorMode,
		HasAlpha:  hasAlpha,
	}, nil
}

// Custom sepia filter
func (p *ImageProcessor) applySepia(img image.Image, intensity float64) image.Image {
	bounds := img.Bounds()
	sepia := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()

			// Convert into 8-bit values
			r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

			/*
				Apply sepia formula (matrix multiply)
				[newR]   [0.393  0.769  0.189]   [originalR]
				[newG] = [0.349  0.686  0.168] * [originalG]
				[newB]   [0.272  0.534  0.131]   [originalB]
			*/
			newR := uint8(float64(r8)*0.393 + float64(g8)*0.769 + float64(b8)*0.189)
			newG := uint8(float64(r8)*0.349 + float64(g8)*0.686 + float64(b8)*0.168)
			newB := uint8(float64(r8)*0.272 + float64(g8)*0.534 + float64(b8)*0.131)

			/*
				Linear interpolation for applying intensity
				Formula: result = original * (1 - intensity) + sepia * intensity
				For example,
				intensity = 0.0 => 100% original + 0% sepia = no effect
				intensity = 0.5 => 50% original + 50% sepia = low effect
				intensity = 1.0 => 0% original + 100% sepia = max effect
			*/
			if intensity < 1.0 {
				newR = uint8(float64(r8)*(1-intensity) + float64(newR)*intensity)
				newG = uint8(float64(g8)*(1-intensity) + float64(newG)*intensity)
				newB = uint8(float64(b8)*(1-intensity) + float64(newB)*intensity)
			}

			sepia.SetRGBA(x, y, color.RGBA{
				R: newR,
				G: newG,
				B: newB,
				A: uint8(a >> 8),
			})
		}
	}

	return sepia
}
