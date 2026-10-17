package domain

import (
	"errors"
	"time"
)

type Measurement struct {
	ID         int64     `json:"id"`
	DeviceID   string    `json:"device_id"`
	Metric     string    `json:"metric"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	RecordedAt time.Time `json:"recorded_at"`
	IngestedAt time.Time `json:"ingested_at"`
}

type TelemetryBatch struct {
	ID             string     `json:"id"`
	DeviceID       string     `json:"device_id"`
	BatchID        string     `json:"batch_id"`
	Status         string     `json:"status"`
	ReceivedCount  int        `json:"received_count"`
	ProcessedCount int        `json:"processed_count"`
	ReceivedAt     time.Time  `json:"received_at"`
	ProcessedAt    *time.Time `json:"processed_at,omitempty"`
}

type TelemetryEvent struct {
	EventID      string        `json:"event_id"`
	DeviceID     string        `json:"device_id"`
	BatchID      string        `json:"batch_id,omitempty"`
	SentAt       time.Time     `json:"sent_at"`
	Measurements []Measurement `json:"measurements"`
}

type AggregatedMeasurement struct {
	DeviceID string    `json:"device_id"`
	Metric   string    `json:"metric"`
	Bucket   time.Time `json:"bucket"`
	Min      float64   `json:"min"`
	Max      float64   `json:"max"`
	Avg      float64   `json:"avg"`
	Count    int64     `json:"count"`
}

type TelemetryStats struct {
	Metric string  `json:"metric"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
	Count  int64   `json:"count"`
}

type DeviceLatestState struct {
	DeviceID   string                 `json:"device_id"`
	Status     DeviceStatus           `json:"status"`
	LastSeenAt *time.Time             `json:"last_seen_at,omitempty"`
	Metrics    map[string]LatestPoint `json:"metrics"`
}

type LatestPoint struct {
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	RecordedAt time.Time `json:"recorded_at"`
}

var (
	ErrEmptyBatch             = errors.New("telemetry batch is empty")
	ErrBatchTooLarge          = errors.New("telemetry batch exceeds max size")
	ErrFutureTimestamp        = errors.New("measurement timestamp is too far in future")
	ErrTooOldTimestamp        = errors.New("measurement timestamp is too old")
	ErrRateLimitExceeded      = errors.New("rate limit exceeded")
	ErrDuplicateMeasurement   = errors.New("duplicate measurement detected")
	ErrInvalidMeasurementData = errors.New("invalid measurement data")
)
