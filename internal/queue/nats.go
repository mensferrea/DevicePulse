package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"DevicePulse/internal/domain"
	"github.com/nats-io/nats.go"
)

type NatsClient struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

func NewNatsClient(url, streamName string) (*NatsClient, error) {
	nc, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("obtain jetstream context: %w", err)
	}

	streamInfo, err := js.StreamInfo(streamName)
	if err != nil || streamInfo == nil {
		_, err = js.AddStream(&nats.StreamConfig{
			Name:        streamName,
			Description: "DevicePulse Telemetry and Alerts Stream",
			Subjects: []string{
				"telemetry.raw",
				"telemetry.raw.*",
				"telemetry.alerts",
				"telemetry.dlq",
			},
			Storage:   nats.FileStorage,
			Retention: nats.LimitsPolicy,
			Discard:   nats.DiscardOld,
			MaxAge:    7 * 24 * time.Hour,
		})
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("create stream %s: %w", streamName, err)
		}
	}

	return &NatsClient{
		nc: nc,
		js: js,
	}, nil
}

func (c *NatsClient) Close() {
	if c.nc != nil {
		_ = c.nc.Drain()
		c.nc.Close()
	}
}

func (c *NatsClient) JetStream() nats.JetStreamContext {
	return c.js
}

func (c *NatsClient) PublishTelemetry(ctx context.Context, subject string, event *domain.TelemetryEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal telemetry event: %w", err)
	}

	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	msg.Header.Set("Nats-Msg-Id", event.EventID)

	_, err = c.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("publish telemetry to %s: %w", subject, err)
	}
	return nil
}

func (c *NatsClient) PublishAlert(ctx context.Context, subject string, alert *domain.AlertNotificationMessage) error {
	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert message: %w", err)
	}

	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	msg.Header.Set("Nats-Msg-Id", fmt.Sprintf("alert-%s-%d", alert.AlertID, alert.Timestamp.UnixNano()))

	_, err = c.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("publish alert to %s: %w", subject, err)
	}
	return nil
}

func (c *NatsClient) PublishDLQ(ctx context.Context, originalSubject string, data []byte, reason string) error {
	msg := &nats.Msg{
		Subject: "telemetry.dlq",
		Data:    data,
		Header:  nats.Header{},
	}
	msg.Header.Set("Original-Subject", originalSubject)
	msg.Header.Set("Failure-Reason", reason)
	msg.Header.Set("Failed-At", time.Now().UTC().Format(time.RFC3339))

	_, err := c.js.PublishMsg(msg, nats.Context(ctx))
	return err
}
