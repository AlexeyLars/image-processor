package processor

type FilterType int

const (
	FilterBlur FilterType = iota
	FilterSharpen
	FilterGrayscale
	FilterSepia
	FilterBrightness
	FilterContrast
)

// ImageInfo содержит информацию об изображении
type ImageInfo struct {
	Width     int32
	Height    int32
	Format    string
	SizeBytes int64
	ColorMode string
	HasAlpha  bool
}
