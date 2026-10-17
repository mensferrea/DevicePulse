package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	IngestedMeasurements *prometheus.CounterVec
	IngestionDuration    *prometheus.HistogramVec
	ProcessedEvents      *prometheus.CounterVec
	ProcessingDuration   *prometheus.HistogramVec
	DuplicateEvents      prometheus.Counter
	RateLimitRejected    prometheus.Counter
	ActiveAlerts         *prometheus.GaugeVec
	AlertEventsTotal     *prometheus.CounterVec
	WebhookDeliveries    *prometheus.CounterVec
	WebhookDuration      *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		IngestedMeasurements: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "devicepulse",
				Subsystem: "ingest",
				Name:      "measurements_total",
				Help:      "Total number of measurements received at ingestion layer",
			},
			[]string{"transport", "status"},
		),
		IngestionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "devicepulse",
				Subsystem: "ingest",
				Name:      "duration_seconds",
				Help:      "Latency of ingestion requests in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"transport"},
		),
		ProcessedEvents: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "devicepulse",
				Subsystem: "processor",
				Name:      "events_total",
				Help:      "Total number of events processed asynchronously",
			},
			[]string{"status"},
		),
		ProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "devicepulse",
				Subsystem: "processor",
				Name:      "duration_seconds",
				Help:      "End-to-end processing duration of an event",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"type"},
		),
		DuplicateEvents: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: "devicepulse",
				Subsystem: "processor",
				Name:      "duplicates_total",
				Help:      "Total number of duplicate measurements detected and ignored",
			},
		),
		RateLimitRejected: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: "devicepulse",
				Subsystem: "ingest",
				Name:      "rate_limited_total",
				Help:      "Total number of requests rejected by rate limiting",
			},
		),
		ActiveAlerts: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "devicepulse",
				Subsystem: "alert",
				Name:      "active_total",
				Help:      "Number of currently active firing alerts",
			},
			[]string{"severity"},
		),
		AlertEventsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "devicepulse",
				Subsystem: "alert",
				Name:      "events_total",
				Help:      "Total count of alert status changes",
			},
			[]string{"status"},
		),
		WebhookDeliveries: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "devicepulse",
				Subsystem: "webhook",
				Name:      "deliveries_total",
				Help:      "Total number of webhook deliveries",
			},
			[]string{"status"},
		),
		WebhookDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "devicepulse",
				Subsystem: "webhook",
				Name:      "duration_seconds",
				Help:      "Latency of webhook HTTP requests in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"status"},
		),
	}
}
