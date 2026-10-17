package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"DevicePulse/internal/config"
	"DevicePulse/internal/domain"
	"DevicePulse/internal/queue"
	"DevicePulse/internal/telemetry"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type ProcessorService struct {
	cfg           *config.Config
	telemetryRepo domain.TelemetryRepository
	deviceRepo    domain.DeviceRepository
	alertRepo     domain.AlertRepository
	stateRepo     domain.StateRepository
	natsClient    *queue.NatsClient
	metrics       *telemetry.Metrics
	workerPool    chan struct{}
}

func NewProcessorService(
	cfg *config.Config,
	tr domain.TelemetryRepository,
	dr domain.DeviceRepository,
	ar domain.AlertRepository,
	sr domain.StateRepository,
	nc *queue.NatsClient,
	m *telemetry.Metrics,
) *ProcessorService {
	poolSize := cfg.WorkerPoolSize
	if poolSize <= 0 {
		poolSize = 10
	}
	return &ProcessorService{
		cfg:           cfg,
		telemetryRepo: tr,
		deviceRepo:    dr,
		alertRepo:     ar,
		stateRepo:     sr,
		natsClient:    nc,
		metrics:       m,
		workerPool:    make(chan struct{}, poolSize),
	}
}

func (s *ProcessorService) Start(ctx context.Context) error {
	js := s.natsClient.JetStream()

	sub, err := js.PullSubscribe(
		"telemetry.raw.*",
		"telemetry-processor-consumer",
		nats.BindStream(s.cfg.NatsStream),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(5),
	)
	if err != nil {
		return fmt.Errorf("subscribe to telemetry.raw.*: %w", err)
	}

	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		default:
			msgs, err := sub.Fetch(10, nats.MaxWait(1*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				slog.Error("fetch telemetry messages error", "error", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			for _, msg := range msgs {
				s.workerPool <- struct{}{}
				wg.Add(1)

				go func(m *nats.Msg) {
					defer func() {
						<-s.workerPool
						wg.Done()
					}()

					s.processMessage(ctx, m)
				}(msg)
			}
		}
	}
}

func (s *ProcessorService) processMessage(ctx context.Context, msg *nats.Msg) {
	timer := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.ProcessingDuration.WithLabelValues("batch").Observe(time.Since(timer).Seconds())
		}
	}()

	var event domain.TelemetryEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.Error("failed to unmarshal telemetry event", "error", err)
		_ = s.natsClient.PublishDLQ(ctx, msg.Subject, msg.Data, fmt.Sprintf("corrupt payload: %v", err))
		_ = msg.Ack()
		if s.metrics != nil {
			s.metrics.ProcessedEvents.WithLabelValues("poison_pill").Inc()
		}
		return
	}

	meta, err := msg.Metadata()
	if err == nil && meta.NumDelivered > 4 {
		slog.Warn("message exceeded max deliveries, routing to DLQ", "event_id", event.EventID)
		_ = s.natsClient.PublishDLQ(ctx, msg.Subject, msg.Data, "max delivery attempts reached")
		_ = msg.Ack()
		if s.metrics != nil {
			s.metrics.ProcessedEvents.WithLabelValues("dlq").Inc()
		}
		return
	}

	var toInsert []domain.Measurement
	var latestTime time.Time

	for _, m := range event.Measurements {
		seen, err := s.stateRepo.IsMeasurementSeen(ctx, m.DeviceID, m.Metric, m.RecordedAt, 24*time.Hour)
		if err == nil && seen {
			if s.metrics != nil {
				s.metrics.DuplicateEvents.Inc()
			}
			continue
		}

		toInsert = append(toInsert, m)
		if m.RecordedAt.After(latestTime) {
			latestTime = m.RecordedAt
		}
	}

	if len(toInsert) > 0 {
		insertedCount, err := s.telemetryRepo.InsertMeasurements(ctx, toInsert)
		if err != nil {
			slog.Error("failed to insert measurements", "device_id", event.DeviceID, "error", err)
			_ = msg.NakWithDelay(2 * time.Second)
			if s.metrics != nil {
				s.metrics.ProcessedEvents.WithLabelValues("db_error").Inc()
			}
			return
		}

		if event.BatchID != "" {
			_ = s.telemetryRepo.UpdateBatchStatus(ctx, event.DeviceID, event.BatchID, "PROCESSED", insertedCount)
		}

		for _, m := range toInsert {
			_ = s.stateRepo.SetLatestPoint(ctx, m.DeviceID, m.Metric, domain.LatestPoint{
				Value:      m.Value,
				Unit:       m.Unit,
				RecordedAt: m.RecordedAt,
			})
		}

		if !latestTime.IsZero() {
			_ = s.stateRepo.SetDeviceStatus(ctx, event.DeviceID, domain.DeviceStatusOnline, latestTime)
			_ = s.deviceRepo.UpdateLastSeen(ctx, event.DeviceID, latestTime, domain.DeviceStatusOnline)
		}

		s.evaluateAlertRules(ctx, event.DeviceID, toInsert)
	}

	if err := msg.Ack(); err != nil {
		slog.Error("failed to ack message", "error", err)
	}

	if s.metrics != nil {
		s.metrics.ProcessedEvents.WithLabelValues("success").Inc()
	}
}

func (s *ProcessorService) evaluateAlertRules(ctx context.Context, deviceID string, measurements []domain.Measurement) {
	metricNames := make(map[string]bool)
	for _, m := range measurements {
		metricNames[m.Metric] = true
	}

	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return
	}

	for metricName := range metricNames {
		rules, err := s.alertRepo.ListRules(ctx, metricName, true)
		if err != nil || len(rules) == 0 {
			continue
		}

		var latestPoint *domain.Measurement
		for i := range measurements {
			if measurements[i].Metric == metricName {
				if latestPoint == nil || measurements[i].RecordedAt.After(latestPoint.RecordedAt) {
					latestPoint = &measurements[i]
				}
			}
		}
		if latestPoint == nil {
			continue
		}

		for _, rule := range rules {
			if rule.DeviceID != nil && *rule.DeviceID != "" && *rule.DeviceID != deviceID {
				continue
			}
			if rule.GroupName != nil && *rule.GroupName != "" && *rule.GroupName != device.Group {
				continue
			}

			activeAlert, err := s.alertRepo.GetActiveAlert(ctx, deviceID, rule.ID)
			if err != nil {
				continue
			}

			conditionMet := EvaluateCondition(rule.Operator, latestPoint.Value, rule.Threshold)

			if conditionMet {
				if activeAlert != nil {
					continue
				}

				now := time.Now().UTC()
				alertID := uuid.New().String()
				msg := FormatAlertMessage(&rule, deviceID, latestPoint.Value)

				newAlert := &domain.Alert{
					ID:           alertID,
					RuleID:       rule.ID,
					DeviceID:     deviceID,
					Metric:       metricName,
					TriggerValue: latestPoint.Value,
					Threshold:    rule.Threshold,
					Status:       domain.AlertStatusFiring,
					Message:      msg,
					FiredAt:      now,
					CreatedAt:    now,
					UpdatedAt:    now,
				}

				if err := s.alertRepo.CreateAlert(ctx, newAlert); err != nil {
					slog.Error("failed to create alert", "error", err)
					continue
				}

				event := &domain.AlertEvent{
					ID:           uuid.New().String(),
					AlertID:      alertID,
					OldStatus:    domain.AlertStatusPending,
					NewStatus:    domain.AlertStatusFiring,
					TriggerValue: latestPoint.Value,
					Reason:       "Threshold breached",
					CreatedAt:    now,
				}
				_ = s.alertRepo.CreateEvent(ctx, event)

				if s.metrics != nil {
					s.metrics.ActiveAlerts.WithLabelValues(string(rule.Severity)).Inc()
					s.metrics.AlertEventsTotal.WithLabelValues(string(domain.AlertStatusFiring)).Inc()
				}

				notification := &domain.AlertNotificationMessage{
					AlertID:      alertID,
					RuleID:       rule.ID,
					DeviceID:     deviceID,
					Metric:       metricName,
					TriggerValue: latestPoint.Value,
					Threshold:    rule.Threshold,
					Status:       domain.AlertStatusFiring,
					Severity:     rule.Severity,
					Message:      msg,
					WebhookURL:   rule.WebhookURL,
					Timestamp:    now,
				}
				_ = s.natsClient.PublishAlert(ctx, s.cfg.NatsSubjectAlert, notification)

			} else {
				if activeAlert != nil {
					now := time.Now().UTC()
					if err := s.alertRepo.UpdateAlertStatus(ctx, activeAlert.ID, domain.AlertStatusResolved, nil, &now); err == nil {
						event := &domain.AlertEvent{
							ID:           uuid.New().String(),
							AlertID:      activeAlert.ID,
							OldStatus:    activeAlert.Status,
							NewStatus:    domain.AlertStatusResolved,
							TriggerValue: latestPoint.Value,
							Reason:       "Metric normalized",
							CreatedAt:    now,
						}
						_ = s.alertRepo.CreateEvent(ctx, event)

						if s.metrics != nil {
							s.metrics.ActiveAlerts.WithLabelValues(string(rule.Severity)).Dec()
							s.metrics.AlertEventsTotal.WithLabelValues(string(domain.AlertStatusResolved)).Inc()
						}

						resolveMsg := fmt.Sprintf("Rule '%s' resolved on device %s: metric '%s' normalized to %.2f",
							rule.Name, deviceID, rule.Metric, latestPoint.Value)

						notification := &domain.AlertNotificationMessage{
							AlertID:      activeAlert.ID,
							RuleID:       rule.ID,
							DeviceID:     deviceID,
							Metric:       metricName,
							TriggerValue: latestPoint.Value,
							Threshold:    rule.Threshold,
							Status:       domain.AlertStatusResolved,
							Severity:     rule.Severity,
							Message:      resolveMsg,
							WebhookURL:   rule.WebhookURL,
							Timestamp:    now,
						}
						_ = s.natsClient.PublishAlert(ctx, s.cfg.NatsSubjectAlert, notification)
					}
				}
			}
		}
	}
}
