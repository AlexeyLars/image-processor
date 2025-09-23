package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for image processing service
type Metrics struct {
	// Request counters
	RequestsTotal    *prometheus.CounterVec
	RequestsInFlight prometheus.Gauge

	// Processing time histograms
	ProcessingDuration *prometheus.HistogramVec

	// Error counters
	ErrorsTotal *prometheus.CounterVec

	// Resource usage
	MemoryUsageBytes prometheus.Gauge
	SemaphoreSlots   prometheus.Gauge
	SemaphoreUsed    prometheus.Gauge

	// Business metrics
	ImageSizeBytes  *prometheus.HistogramVec
	FiltersApplied  *prometheus.CounterVec
	ProcessingQueue prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "image_processing_requests_total",
				Help: "Total number of image processing requests",
			},
			[]string{"method", "status"}, // method: apply_filter, resize_image, etc; status: success, failed
		),

		RequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "image_processing_requests_in_flight",
				Help: "Number of image processing requests currently being handled",
			},
		),

		ProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "image_processing_duration_seconds",
				Help:    "Time spent processing images",
				Buckets: prometheus.DefBuckets, // [.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10]
			},
			[]string{"method"}, // apply_filter, resize_image, etc
		),

		ErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "image_processing_errors_total",
				Help: "Total number of image processing errors",
			},
			[]string{"method", "error_type"}, // error_type: decode_error, processing_error, timeout, etc
		),

		MemoryUsageBytes: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "image_processing_memory_usage_bytes",
				Help: "Current memory usage by semaphore in bytes",
			},
		),

		SemaphoreSlots: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "image_processing_semaphore_slots_total",
				Help: "Total number of semaphore slots available",
			},
		),

		SemaphoreUsed: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "image_processing_semaphore_slots_used",
				Help: "Number of semaphore slots currently in use",
			},
		),

		ImageSizeBytes: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "image_processing_image_size_bytes",
				Help:    "Size of images being processed",
				Buckets: []float64{1024, 10240, 102400, 1024000, 10240000, 52428800, 104857600}, // 1KB to 100MB
			},
			[]string{"direction"}, // input, output
		),

		FiltersApplied: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "image_processing_filters_applied_total",
				Help: "Total number of filters applied by type",
			},
			[]string{"filter_type"}, // blur, sharpen, grayscale, etc
		),

		ProcessingQueue: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "image_processing_queue_length",
				Help: "Number of requests waiting in the processing queue",
			},
		),
	}
}

// RecordRequestStart increments the request counter and in-flight gauge
func (m *Metrics) RecordRequestStart(method string) {
	m.RequestsInFlight.Inc()
	m.ProcessingQueue.Dec() // Assuming request moved from queue to processing
}

// RecordRequestEnd decrements in-flight gauge and records the final status
func (m *Metrics) RecordRequestEnd(method, status string) {
	m.RequestsInFlight.Dec()
	m.RequestsTotal.WithLabelValues(method, status).Inc()
}

// RecordError increments the error counter
func (m *Metrics) RecordError(method, errorType string) {
	m.ErrorsTotal.WithLabelValues(method, errorType).Inc()
}

// RecordProcessingDuration records how long processing took
func (m *Metrics) RecordProcessingDuration(method string, seconds float64) {
	m.ProcessingDuration.WithLabelValues(method).Observe(seconds)
}

// RecordImageSize records the size of IO images
func (m *Metrics) RecordImageSize(direction string, sizeBytes int64) {
	m.ImageSizeBytes.WithLabelValues(direction).Observe(float64(sizeBytes))
}

// RecordFilterApplied increments the counter for a specific filter type
func (m *Metrics) RecordFilterApplied(filterType string) {
	m.FiltersApplied.WithLabelValues(filterType).Inc()
}

// UpdateSemaphoreUsage updates the semaphore usage metrics
func (m *Metrics) UpdateSemaphoreUsage(used, total int64) {
	m.SemaphoreUsed.Set(float64(used))
	m.SemaphoreSlots.Set(float64(total))
	m.MemoryUsageBytes.Set(float64(used)) // Assuming used represents bytes
}

// UpdateQueueLength updates the processing queue length
func (m *Metrics) UpdateQueueLength(length int64) {
	m.ProcessingQueue.Set(float64(length))
}
