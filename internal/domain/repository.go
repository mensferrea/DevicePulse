package domain

import (
	"context"
	"time"
)

type DeviceFilter struct {
	Group  string
	Status DeviceStatus
	Limit  int
	Offset int
}

type MeasurementFilter struct {
	DeviceID string
	Metric   string
	From     time.Time
	To       time.Time
	Limit    int
	Offset   int
}

type AggregationFilter struct {
	DeviceID string
	Metric   string
	Interval string
	From     time.Time
	To       time.Time
}

type AlertFilter struct {
	DeviceID string
	RuleID   string
	Status   AlertStatus
	Limit    int
	Offset   int
}

type DeviceRepository interface {
	Create(ctx context.Context, device *Device) error
	GetByID(ctx context.Context, id string) (*Device, error)
	List(ctx context.Context, filter DeviceFilter) ([]Device, error)
	Update(ctx context.Context, device *Device) error
	Delete(ctx context.Context, id string) error
	UpdateLastSeen(ctx context.Context, deviceID string, lastSeenAt time.Time, status DeviceStatus) error
	CreateCredential(ctx context.Context, cred *DeviceCredential) error
	GetCredentialByHash(ctx context.Context, keyHash string) (*DeviceCredential, error)
	ListCredentials(ctx context.Context, deviceID string) ([]DeviceCredential, error)
	RevokeCredential(ctx context.Context, credentialID string) error
	GetOfflineCandidates(ctx context.Context, threshold time.Duration) ([]Device, error)
}

type MetricRepository interface {
	Create(ctx context.Context, metric *MetricDefinition) error
	GetByName(ctx context.Context, name string) (*MetricDefinition, error)
	List(ctx context.Context) ([]MetricDefinition, error)
	Delete(ctx context.Context, id string) error
}

type TelemetryRepository interface {
	CreateBatchRecord(ctx context.Context, batch *TelemetryBatch) error
	UpdateBatchStatus(ctx context.Context, deviceID, batchID, status string, processedCount int) error
	InsertMeasurements(ctx context.Context, measurements []Measurement) (int, error)
	GetMeasurements(ctx context.Context, filter MeasurementFilter) ([]Measurement, error)
	GetAggregated(ctx context.Context, filter AggregationFilter) ([]AggregatedMeasurement, error)
	GetStats(ctx context.Context, deviceID, metric string, from, to time.Time) (*TelemetryStats, error)
}

type AlertRepository interface {
	CreateRule(ctx context.Context, rule *AlertRule) error
	GetRuleByID(ctx context.Context, id string) (*AlertRule, error)
	ListRules(ctx context.Context, metric string, activeOnly bool) ([]AlertRule, error)
	UpdateRule(ctx context.Context, rule *AlertRule) error
	DeleteRule(ctx context.Context, id string) error
	GetActiveAlert(ctx context.Context, deviceID, ruleID string) (*Alert, error)
	CreateAlert(ctx context.Context, alert *Alert) error
	UpdateAlertStatus(ctx context.Context, alertID string, status AlertStatus, ackAt, resAt *time.Time) error
	CreateEvent(ctx context.Context, event *AlertEvent) error
	ListAlerts(ctx context.Context, filter AlertFilter) ([]Alert, error)
	GetAlertByID(ctx context.Context, id string) (*Alert, error)
}

type WebhookRepository interface {
	CreateDelivery(ctx context.Context, delivery *WebhookDelivery) error
	UpdateDeliveryStatus(ctx context.Context, id string, status WebhookDeliveryStatus, code *int, errMsg *string, deliveredAt *time.Time) error
	ListDeliveries(ctx context.Context, alertID string) ([]WebhookDelivery, error)
}

type StateRepository interface {
	SetLatestPoint(ctx context.Context, deviceID, metric string, point LatestPoint) error
	GetLatestState(ctx context.Context, deviceID string) (*DeviceLatestState, error)
	SetDeviceStatus(ctx context.Context, deviceID string, status DeviceStatus, lastSeen time.Time) error
	GetDeviceStatus(ctx context.Context, deviceID string) (DeviceStatus, *time.Time, error)
	CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
	IsMeasurementSeen(ctx context.Context, deviceID, metric string, recordedAt time.Time, ttl time.Duration) (bool, error)
}
