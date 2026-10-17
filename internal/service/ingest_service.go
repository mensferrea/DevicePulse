package service

import (
	"context"
	"fmt"
	"time"

	"DevicePulse/internal/config"
	"DevicePulse/internal/domain"
	"DevicePulse/internal/queue"
	"DevicePulse/internal/telemetry"
	"github.com/google/uuid"
)

type IngestService struct {
	cfg           *config.Config
	telemetryRepo domain.TelemetryRepository
	metricRepo    domain.MetricRepository
	stateRepo     domain.StateRepository
	natsClient    *queue.NatsClient
	metrics       *telemetry.Metrics
}

func NewIngestService(
	cfg *config.Config,
	tr domain.TelemetryRepository,
	mr domain.MetricRepository,
	sr domain.StateRepository,
	nc *queue.NatsClient,
	m *telemetry.Metrics,
) *IngestService {
	return &IngestService{
		cfg:           cfg,
		telemetryRepo: tr,
		metricRepo:    mr,
		stateRepo:     sr,
		natsClient:    nc,
		metrics:       m,
	}
}

func (s *IngestService) Ingest(ctx context.Context, deviceID, batchID string, measurements []domain.Measurement, transport string) error {
	timer := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.IngestionDuration.WithLabelValues(transport).Observe(time.Since(timer).Seconds())
		}
	}()

	if len(measurements) == 0 {
		return domain.ErrEmptyBatch
	}

	if len(measurements) > s.cfg.MaxBatchSize {
		return domain.ErrBatchTooLarge
	}

	allowed, err := s.stateRepo.CheckRateLimit(ctx, deviceID, s.cfg.RateLimitMaxRPS, time.Second)
	if err == nil && !allowed {
		if s.metrics != nil {
			s.metrics.RateLimitRejected.Inc()
			s.metrics.IngestedMeasurements.WithLabelValues(transport, "rate_limited").Add(float64(len(measurements)))
		}
		return domain.ErrRateLimitExceeded
	}

	now := time.Now().UTC()
	for i := range measurements {
		m := &measurements[i]
		m.DeviceID = deviceID

		if m.RecordedAt.IsZero() {
			m.RecordedAt = now
		}

		if m.RecordedAt.After(now.Add(10 * time.Minute)) {
			return domain.ErrFutureTimestamp
		}
		if m.RecordedAt.Before(now.Add(-30 * 24 * time.Hour)) {
			return domain.ErrTooOldTimestamp
		}
		if m.Metric == "" {
			return domain.ErrInvalidMeasurementData
		}

		m.IngestedAt = now
	}

	if batchID != "" {
		batch := &domain.TelemetryBatch{
			ID:            uuid.New().String(),
			DeviceID:      deviceID,
			BatchID:       batchID,
			Status:        "RECEIVED",
			ReceivedCount: len(measurements),
			ReceivedAt:    now,
		}
		_ = s.telemetryRepo.CreateBatchRecord(ctx, batch)
	}

	event := &domain.TelemetryEvent{
		EventID:      uuid.New().String(),
		DeviceID:     deviceID,
		BatchID:      batchID,
		SentAt:       now,
		Measurements: measurements,
	}

	subject := fmt.Sprintf("%s.%s", s.cfg.NatsSubjectRaw, deviceID)
	if err := s.natsClient.PublishTelemetry(ctx, subject, event); err != nil {
		if s.metrics != nil {
			s.metrics.IngestedMeasurements.WithLabelValues(transport, "error").Add(float64(len(measurements)))
		}
		return fmt.Errorf("publish event to queue: %w", err)
	}

	if s.metrics != nil {
		s.metrics.IngestedMeasurements.WithLabelValues(transport, "success").Add(float64(len(measurements)))
	}

	return nil
}
