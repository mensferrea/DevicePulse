package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"DevicePulse/internal/config"
	"DevicePulse/internal/domain"
	"DevicePulse/internal/queue"
	"DevicePulse/internal/telemetry"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type WebhookService struct {
	cfg         *config.Config
	webhookRepo domain.WebhookRepository
	natsClient  *queue.NatsClient
	metrics     *telemetry.Metrics
	httpClient  *http.Client
}

func NewWebhookService(
	cfg *config.Config,
	wr domain.WebhookRepository,
	nc *queue.NatsClient,
	m *telemetry.Metrics,
) *WebhookService {
	return &WebhookService{
		cfg:         cfg,
		webhookRepo: wr,
		natsClient:  nc,
		metrics:     m,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *WebhookService) Start(ctx context.Context) error {
	js := s.natsClient.JetStream()

	sub, err := js.PullSubscribe(
		s.cfg.NatsSubjectAlert,
		"alert-webhook-consumer",
		nats.BindStream(s.cfg.NatsStream),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(5),
	)
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", s.cfg.NatsSubjectAlert, err)
	}

	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		default:
			msgs, err := sub.Fetch(5, nats.MaxWait(1*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				slog.Error("fetch alert messages error", "error", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			for _, msg := range msgs {
				wg.Add(1)
				go func(m *nats.Msg) {
					defer wg.Done()
					s.processAlertMessage(ctx, m)
				}(msg)
			}
		}
	}
}

func (s *WebhookService) processAlertMessage(ctx context.Context, msg *nats.Msg) {
	var alertMsg domain.AlertNotificationMessage
	if err := json.Unmarshal(msg.Data, &alertMsg); err != nil {
		slog.Error("failed to unmarshal alert notification", "error", err)
		_ = msg.Ack()
		return
	}

	if alertMsg.WebhookURL == "" {
		_ = msg.Ack()
		return
	}

	deliveryID := uuid.New().String()
	delivery := &domain.WebhookDelivery{
		ID:        deliveryID,
		AlertID:   alertMsg.AlertID,
		URL:       alertMsg.WebhookURL,
		Attempt:   1,
		Status:    domain.WebhookStatusPending,
		CreatedAt: time.Now().UTC(),
	}
	_ = s.webhookRepo.CreateDelivery(ctx, delivery)

	payloadBytes, err := json.Marshal(alertMsg)
	if err != nil {
		errMsg := err.Error()
		_ = s.webhookRepo.UpdateDeliveryStatus(ctx, deliveryID, domain.WebhookStatusFailed, nil, &errMsg, nil)
		_ = msg.Ack()
		return
	}

	maxRetries := 3
	var lastErr error
	var respCode int
	success := false

	timer := time.Now()
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, alertMsg.WebhookURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			lastErr = err
			break
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "DevicePulse-Alert-Worker/1.0")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			continue
		}

		respCode = resp.StatusCode
		_ = resp.Body.Close()

		if respCode >= 200 && respCode < 300 {
			success = true
			break
		} else if respCode >= 400 && respCode < 500 {
			lastErr = fmt.Errorf("client error: %d", respCode)
			break
		} else {
			lastErr = fmt.Errorf("server error: %d", respCode)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
	}

	dur := time.Since(timer).Seconds()
	now := time.Now().UTC()

	if success {
		_ = s.webhookRepo.UpdateDeliveryStatus(ctx, deliveryID, domain.WebhookStatusSuccess, &respCode, nil, &now)
		if s.metrics != nil {
			s.metrics.WebhookDeliveries.WithLabelValues("success").Inc()
			s.metrics.WebhookDuration.WithLabelValues("success").Observe(dur)
		}
	} else {
		var errStr string
		if lastErr != nil {
			errStr = lastErr.Error()
		}
		var codePtr *int
		if respCode > 0 {
			codePtr = &respCode
		}
		_ = s.webhookRepo.UpdateDeliveryStatus(ctx, deliveryID, domain.WebhookStatusFailed, codePtr, &errStr, &now)
		if s.metrics != nil {
			s.metrics.WebhookDeliveries.WithLabelValues("failed").Inc()
			s.metrics.WebhookDuration.WithLabelValues("failed").Observe(dur)
		}
	}

	_ = msg.Ack()
}
