package processor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestNewImageProcessor(t *testing.T) {
	processor := NewImageProcessor()

	if processor == nil {
		t.Fatal("NewImageProcessor() returned nil")
	}

	t.Logf("ImageProcessor created successfully")
}

func TestDecodeImage(t *testing.T) {
	processor := NewImageProcessor()

	width, height := 100, 100
	testImg := createTestImage(width, height, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	testData := encodeImageToPNG(t, testImg)

	decodedImg, format, err := processor.DecodeImage(testData)

	if err != nil {
		t.Fatalf("DecodeImage() failed: %v", err)
	}

	if format != "png" {
		t.Errorf("Expected format 'png', got '%s'", format)
	}

	if decodedImg == nil {
		t.Fatal("Decoded image is nil")
	}

	bounds := decodedImg.Bounds()
	if bounds.Dx() != width || bounds.Dy() != height {
		t.Errorf("Expected image size %dx%d, got %dx%d", width, height, bounds.Dx(), bounds.Dy())
	}

	t.Logf("Successfully decoded %s image of size %dx%d", format, bounds.Dx(), bounds.Dy())
}

func TestDecodeImage_InvalidData(t *testing.T) {
	processor := NewImageProcessor()

	invalidData := []byte("not image")

	_, _, err := processor.DecodeImage(invalidData)

	if err == nil {
		t.Error("Expected error for invalid image data, but got nil")
	}

	t.Logf("Correctly handled invalid data with error: %v", err)
}

func TestEncodeImage(t *testing.T) {
	processor := NewImageProcessor()

	testImg := createTestImage(121, 310, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	tests := []struct {
		name   string
		format string
	}{
		{"PNG format", "png"},
		{"JPEG format", "jpeg"},
		{"JPG format", "jpg"},
		{"Unknown format (should default to PNG)", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := processor.EncodeImage(testImg, tt.format)

			if err != nil {
				t.Fatalf("EncodeImage() failed for format %s: %v", tt.format, err)
			}

			if len(data) == 0 {
				t.Errorf("EncodeImage() returned empty data for format %s", tt.format)
			}

			t.Logf("Successfully encoded image to %s format, size: %d bytes", tt.format, len(data))
		})
	}
}

func TestApplyFilter(t *testing.T) {
	processor := NewImageProcessor()

	testData, err := os.ReadFile("testdata/sample.jpg")
	if err != nil {
		t.Fatalf("Failed to load test image: %v", err)
	}
	testImg, _, err := processor.DecodeImage(testData)
	if err != nil {
		t.Fatalf("Failed to decode test image: %v", err)
	}

	tests := []struct {
		name         string
		filterType   FilterType
		intensity    float64
		shouldChange bool
	}{
		{"Blur filter", FilterBlur, 0.5, true},
		{"Sharpen filter", FilterSharpen, 0.5, true},
		{"Grayscale filter", FilterGrayscale, 1.0, true},
		{"Sepia filter", FilterSepia, 0.8, true},
		{"Brightness filter", FilterBrightness, 0.3, true},
		{"Contrast filter", FilterContrast, 0.3, true},
		{"No effect (sepia 0.0)", FilterSepia, 0.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.ApplyFilter(testImg, tt.filterType, tt.intensity)

			if result == nil {
				t.Fatalf("ApplyFilter() returned nil for filter %v", tt.filterType)
			}

			originalBounds := testImg.Bounds()
			resultBounds := result.Bounds()

			if originalBounds.Dx() != resultBounds.Dx() || originalBounds.Dy() != resultBounds.Dy() {
				t.Errorf("Filter changed image size from %dx%d to %dx%d",
					originalBounds.Dx(), originalBounds.Dy(),
					resultBounds.Dx(), resultBounds.Dy())
			}

			if tt.shouldChange {
				if imagesEqual(testImg, result) {
					t.Errorf("Filter %v with intensity %f should have changed the image, but it didn't",
						tt.filterType, tt.intensity)
				}
			}

			t.Logf("Successfully applied filter %v with intensity %f", tt.filterType, tt.intensity)
			resultImg, err := processor.EncodeImage(result, "jpg")
			if err != nil {
				t.Fatalf("Failed to encode image: %v", err)
			}
			err = os.WriteFile(
				fmt.Sprintf("testdata/results/sample_%s_%f.jpg", tt.name, tt.intensity),
				resultImg, 0644)
			if err != nil {
				t.Fatalf("Failed to write image: %v", err)
			}
		})
	}
}

func TestResizeImage(t *testing.T) {
	processor := NewImageProcessor()

	testImg := createTestImage(200, 200, color.RGBA{R: 255, G: 255, B: 0, A: 255})

	tests := []struct {
		name            string
		width           int
		height          int
		keepAspectRatio bool
		expectedWidth   int
		expectedHeight  int
	}{
		{"Resize to 100x100 without keeping aspect ratio", 100, 100, false, 100, 100},
		{"Resize to 100x50 without keeping aspect ratio", 100, 50, false, 100, 50},
		{"Resize to 100x100 keeping aspect ratio", 100, 100, true, 100, 100},
		{"Resize to 300x150 keeping aspect ratio", 300, 150, true, 150, 150}, // должно быть 150x150 (fit в 300x150)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.ResizeImage(testImg, tt.width, tt.height, tt.keepAspectRatio)

			if result == nil {
				t.Fatalf("ResizeImage() returned nil")
			}

			bounds := result.Bounds()
			actualWidth := bounds.Dx()
			actualHeight := bounds.Dy()

			if tt.keepAspectRatio {
				if actualWidth > tt.width || actualHeight > tt.height {
					t.Errorf("Resized image %dx%d exceeds limits %dx%d",
						actualWidth, actualHeight, tt.width, tt.height)
				}
			} else {
				if actualWidth != tt.expectedWidth || actualHeight != tt.expectedHeight {
					t.Errorf("Expected size %dx%d, got %dx%d",
						tt.expectedWidth, tt.expectedHeight, actualWidth, actualHeight)
				}
			}

			t.Logf("Resize successful: %dx%d -> %dx%d (keepAspect: %v)",
				200, 200, actualWidth, actualHeight, tt.keepAspectRatio)
		})
	}
}

func TestGetImageInfo(t *testing.T) {
	processor := NewImageProcessor()

	testImg := createTestImage(150, 100, color.RGBA{R: 255, G: 128, B: 64, A: 255})
	testData := encodeImageToPNG(t, testImg)

	info, err := processor.GetImageInfo(testData)

	if err != nil {
		t.Fatalf("GetImageInfo() failed: %v", err)
	}

	if info == nil {
		t.Fatal("GetImageInfo() returned nil info")
	}

	if info.Width != 150 {
		t.Errorf("Expected width 150, got %d", info.Width)
	}

	if info.Height != 100 {
		t.Errorf("Expected height 100, got %d", info.Height)
	}

	if info.Format != "png" {
		t.Errorf("Expected format 'png', got '%s'", info.Format)
	}

	if info.SizeBytes != int64(len(testData)) {
		t.Errorf("Expected size %d bytes, got %d", len(testData), info.SizeBytes)
	}

	if info.ColorMode != "RGBA" {
		t.Errorf("Expected color mode 'RGBA', got '%s'", info.ColorMode)
	}

	if !info.HasAlpha {
		t.Error("Expected HasAlpha to be true for RGBA image")
	}

	t.Logf("GetImageInfo successful: %dx%d %s, %d bytes, %s mode, alpha: %v",
		info.Width, info.Height, info.Format, info.SizeBytes, info.ColorMode, info.HasAlpha)
}

func createTestImage(width, height int, fillColor color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, fillColor)
		}
	}

	return img
}

func encodeImageToPNG(t *testing.T, img image.Image) []byte {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		t.Fatalf("Failed to encode test image to PNG: %v", err)
	}
	return buf.Bytes()
}

func imagesEqual(img1, img2 image.Image) bool {
	bounds1 := img1.Bounds()
	bounds2 := img2.Bounds()

	if bounds1 != bounds2 {
		return false
	}

	// Part-check pixels
	for y := bounds1.Min.Y; y < bounds1.Max.Y; y += 10 {
		for x := bounds1.Min.X; x < bounds1.Max.X; x += 10 {
			r1, g1, b1, a1 := img1.At(x, y).RGBA()
			r2, g2, b2, a2 := img2.At(x, y).RGBA()

			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}

	return true
}
