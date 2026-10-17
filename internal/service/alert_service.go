package service

import (
	"context"
	"fmt"
	"time"

	"DevicePulse/internal/domain"
	"github.com/google/uuid"
)

type AlertService struct {
	alertRepo domain.AlertRepository
}

func NewAlertService(alertRepo domain.AlertRepository) *AlertService {
	return &AlertService{alertRepo: alertRepo}
}

func (s *AlertService) CreateRule(ctx context.Context, rule *domain.AlertRule) error {
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	rule.UpdatedAt = now
	return s.alertRepo.CreateRule(ctx, rule)
}

func (s *AlertService) GetRule(ctx context.Context, id string) (*domain.AlertRule, error) {
	return s.alertRepo.GetRuleByID(ctx, id)
}

func (s *AlertService) ListRules(ctx context.Context, metric string, activeOnly bool) ([]domain.AlertRule, error) {
	return s.alertRepo.ListRules(ctx, metric, activeOnly)
}

func (s *AlertService) UpdateRule(ctx context.Context, rule *domain.AlertRule) error {
	return s.alertRepo.UpdateRule(ctx, rule)
}

func (s *AlertService) DeleteRule(ctx context.Context, id string) error {
	return s.alertRepo.DeleteRule(ctx, id)
}

func (s *AlertService) ListAlerts(ctx context.Context, filter domain.AlertFilter) ([]domain.Alert, error) {
	return s.alertRepo.ListAlerts(ctx, filter)
}

func (s *AlertService) GetAlert(ctx context.Context, id string) (*domain.Alert, error) {
	return s.alertRepo.GetAlertByID(ctx, id)
}

func (s *AlertService) AcknowledgeAlert(ctx context.Context, id string) error {
	alert, err := s.alertRepo.GetAlertByID(ctx, id)
	if err != nil {
		return err
	}

	if alert.Status == domain.AlertStatusAcknowledged {
		return domain.ErrAlertAlreadyAcked
	}
	if alert.Status == domain.AlertStatusResolved {
		return domain.ErrAlertResolved
	}

	now := time.Now().UTC()
	if err := s.alertRepo.UpdateAlertStatus(ctx, id, domain.AlertStatusAcknowledged, &now, nil); err != nil {
		return err
	}

	event := &domain.AlertEvent{
		ID:           uuid.New().String(),
		AlertID:      id,
		OldStatus:    alert.Status,
		NewStatus:    domain.AlertStatusAcknowledged,
		TriggerValue: alert.TriggerValue,
		Reason:       "Acknowledged by user",
		CreatedAt:    now,
	}
	return s.alertRepo.CreateEvent(ctx, event)
}

func (s *AlertService) ResolveAlert(ctx context.Context, id string, reason string) error {
	alert, err := s.alertRepo.GetAlertByID(ctx, id)
	if err != nil {
		return err
	}

	if alert.Status == domain.AlertStatusResolved {
		return domain.ErrAlertResolved
	}

	now := time.Now().UTC()
	if err := s.alertRepo.UpdateAlertStatus(ctx, id, domain.AlertStatusResolved, nil, &now); err != nil {
		return err
	}

	if reason == "" {
		reason = "Manually resolved"
	}

	event := &domain.AlertEvent{
		ID:           uuid.New().String(),
		AlertID:      id,
		OldStatus:    alert.Status,
		NewStatus:    domain.AlertStatusResolved,
		TriggerValue: alert.TriggerValue,
		Reason:       reason,
		CreatedAt:    now,
	}
	return s.alertRepo.CreateEvent(ctx, event)
}

func EvaluateCondition(op domain.AlertOperator, val, threshold float64) bool {
	switch op {
	case domain.OperatorGT:
		return val > threshold
	case domain.OperatorGTE:
		return val >= threshold
	case domain.OperatorLT:
		return val < threshold
	case domain.OperatorLTE:
		return val <= threshold
	case domain.OperatorEQ:
		return val == threshold
	case domain.OperatorNEQ:
		return val != threshold
	default:
		return false
	}
}

func FormatAlertMessage(rule *domain.AlertRule, deviceID string, val float64) string {
	return fmt.Sprintf("Rule '%s' violated on device %s: metric '%s' value %.2f is %s threshold %.2f",
		rule.Name, deviceID, rule.Metric, val, rule.Operator, rule.Threshold)
}
