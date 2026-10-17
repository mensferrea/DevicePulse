package domain

import (
	"errors"
	"time"
)

type AlertStatus string

const (
	AlertStatusPending      AlertStatus = "PENDING"
	AlertStatusFiring       AlertStatus = "FIRING"
	AlertStatusAcknowledged AlertStatus = "ACKNOWLEDGED"
	AlertStatusResolved     AlertStatus = "RESOLVED"
)

type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

type AlertOperator string

const (
	OperatorGT  AlertOperator = ">"
	OperatorGTE AlertOperator = ">="
	OperatorLT  AlertOperator = "<"
	OperatorLTE AlertOperator = "<="
	OperatorEQ  AlertOperator = "=="
	OperatorNEQ AlertOperator = "!="
)

type AlertRule struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	DeviceID        *string       `json:"device_id,omitempty"`
	GroupName       *string       `json:"group_name,omitempty"`
	Metric          string        `json:"metric"`
	Operator        AlertOperator `json:"operator"`
	Threshold       float64       `json:"threshold"`
	DurationSeconds int           `json:"duration_seconds"`
	Severity        AlertSeverity `json:"severity"`
	Active          bool          `json:"active"`
	WebhookURL      string        `json:"webhook_url"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

type Alert struct {
	ID             string      `json:"id"`
	RuleID         string      `json:"rule_id"`
	DeviceID       string      `json:"device_id"`
	Metric         string      `json:"metric"`
	TriggerValue   float64     `json:"trigger_value"`
	Threshold      float64     `json:"threshold"`
	Status         AlertStatus `json:"status"`
	FiredAt        time.Time   `json:"fired_at"`
	AcknowledgedAt *time.Time  `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time  `json:"resolved_at,omitempty"`
	Message        string      `json:"message"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type AlertEvent struct {
	ID           string      `json:"id"`
	AlertID      string      `json:"alert_id"`
	OldStatus    AlertStatus `json:"old_status"`
	NewStatus    AlertStatus `json:"new_status"`
	TriggerValue float64     `json:"trigger_value"`
	Reason       string      `json:"reason"`
	CreatedAt    time.Time   `json:"created_at"`
}

type WebhookDeliveryStatus string

const (
	WebhookStatusPending WebhookDeliveryStatus = "PENDING"
	WebhookStatusSuccess WebhookDeliveryStatus = "SUCCESS"
	WebhookStatusFailed  WebhookDeliveryStatus = "FAILED"
)

type WebhookDelivery struct {
	ID           string                `json:"id"`
	AlertID      string                `json:"alert_id"`
	AlertEventID string                `json:"alert_event_id"`
	URL          string                `json:"url"`
	Attempt      int                   `json:"attempt"`
	Status       WebhookDeliveryStatus `json:"status"`
	ResponseCode *int                  `json:"response_code,omitempty"`
	ErrorMessage *string               `json:"error_message,omitempty"`
	DeliveredAt  *time.Time            `json:"delivered_at,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
}

type AlertNotificationMessage struct {
	AlertID      string        `json:"alert_id"`
	RuleID       string        `json:"rule_id"`
	DeviceID     string        `json:"device_id"`
	Metric       string        `json:"metric"`
	TriggerValue float64       `json:"trigger_value"`
	Threshold    float64       `json:"threshold"`
	Status       AlertStatus   `json:"status"`
	Severity     AlertSeverity `json:"severity"`
	Message      string        `json:"message"`
	WebhookURL   string        `json:"webhook_url"`
	Timestamp    time.Time     `json:"timestamp"`
}

var (
	ErrAlertRuleNotFound = errors.New("alert rule not found")
	ErrAlertNotFound     = errors.New("alert not found")
	ErrAlertAlreadyAcked = errors.New("alert is already acknowledged")
	ErrAlertResolved     = errors.New("alert is already resolved")
)
