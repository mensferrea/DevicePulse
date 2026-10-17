package service_test

import (
	"context"
	"testing"
	"time"

	"DevicePulse/internal/domain"
	"DevicePulse/internal/service"
)

type mockAlertRepo struct {
	rules  map[string]*domain.AlertRule
	alerts map[string]*domain.Alert
	events []*domain.AlertEvent
}

func newMockAlertRepo() *mockAlertRepo {
	return &mockAlertRepo{
		rules:  make(map[string]*domain.AlertRule),
		alerts: make(map[string]*domain.Alert),
	}
}

func (m *mockAlertRepo) CreateRule(ctx context.Context, rule *domain.AlertRule) error {
	m.rules[rule.ID] = rule
	return nil
}

func (m *mockAlertRepo) GetRuleByID(ctx context.Context, id string) (*domain.AlertRule, error) {
	rule, ok := m.rules[id]
	if !ok {
		return nil, domain.ErrAlertRuleNotFound
	}
	return rule, nil
}

func (m *mockAlertRepo) ListRules(ctx context.Context, metric string, activeOnly bool) ([]domain.AlertRule, error) {
	var res []domain.AlertRule
	for _, r := range m.rules {
		if (metric == "" || r.Metric == metric) && (!activeOnly || r.Active) {
			res = append(res, *r)
		}
	}
	return res, nil
}

func (m *mockAlertRepo) UpdateRule(ctx context.Context, rule *domain.AlertRule) error {
	if _, ok := m.rules[rule.ID]; !ok {
		return domain.ErrAlertRuleNotFound
	}
	m.rules[rule.ID] = rule
	return nil
}

func (m *mockAlertRepo) DeleteRule(ctx context.Context, id string) error {
	if _, ok := m.rules[id]; !ok {
		return domain.ErrAlertRuleNotFound
	}
	delete(m.rules, id)
	return nil
}

func (m *mockAlertRepo) GetActiveAlert(ctx context.Context, deviceID, ruleID string) (*domain.Alert, error) {
	for _, a := range m.alerts {
		if a.DeviceID == deviceID && a.RuleID == ruleID &&
			(a.Status == domain.AlertStatusPending || a.Status == domain.AlertStatusFiring || a.Status == domain.AlertStatusAcknowledged) {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockAlertRepo) CreateAlert(ctx context.Context, alert *domain.Alert) error {
	m.alerts[alert.ID] = alert
	return nil
}

func (m *mockAlertRepo) UpdateAlertStatus(ctx context.Context, alertID string, status domain.AlertStatus, ackAt, resAt *time.Time) error {
	a, ok := m.alerts[alertID]
	if !ok {
		return domain.ErrAlertNotFound
	}
	a.Status = status
	a.AcknowledgedAt = ackAt
	a.ResolvedAt = resAt
	return nil
}

func (m *mockAlertRepo) CreateEvent(ctx context.Context, event *domain.AlertEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockAlertRepo) ListAlerts(ctx context.Context, filter domain.AlertFilter) ([]domain.Alert, error) {
	var res []domain.Alert
	for _, a := range m.alerts {
		if (filter.DeviceID == "" || a.DeviceID == filter.DeviceID) &&
			(filter.Status == "" || a.Status == filter.Status) {
			res = append(res, *a)
		}
	}
	return res, nil
}

func (m *mockAlertRepo) GetAlertByID(ctx context.Context, id string) (*domain.Alert, error) {
	a, ok := m.alerts[id]
	if !ok {
		return nil, domain.ErrAlertNotFound
	}
	return a, nil
}

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		name      string
		op        domain.AlertOperator
		val       float64
		threshold float64
		expected  bool
	}{
		{"gt true", domain.OperatorGT, 85.0, 80.0, true},
		{"gt false", domain.OperatorGT, 75.0, 80.0, false},
		{"gte true equal", domain.OperatorGTE, 80.0, 80.0, true},
		{"gte true greater", domain.OperatorGTE, 81.0, 80.0, true},
		{"gte false", domain.OperatorGTE, 79.9, 80.0, false},
		{"lt true", domain.OperatorLT, 15.0, 20.0, true},
		{"lt false", domain.OperatorLT, 25.0, 20.0, false},
		{"lte true equal", domain.OperatorLTE, 20.0, 20.0, true},
		{"lte false", domain.OperatorLTE, 20.1, 20.0, false},
		{"eq true", domain.OperatorEQ, 100.0, 100.0, true},
		{"eq false", domain.OperatorEQ, 99.0, 100.0, false},
		{"neq true", domain.OperatorNEQ, 99.0, 100.0, true},
		{"neq false", domain.OperatorNEQ, 100.0, 100.0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := service.EvaluateCondition(tc.op, tc.val, tc.threshold)
			if result != tc.expected {
				t.Errorf("expected %v, got %v for val=%.2f %s threshold=%.2f", tc.expected, result, tc.val, tc.op, tc.threshold)
			}
		})
	}
}

func TestAlertLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newMockAlertRepo()
	svc := service.NewAlertService(repo)

	rule := &domain.AlertRule{
		Name:      "High Temperature",
		Metric:    "temperature",
		Operator:  domain.OperatorGT,
		Threshold: 80.0,
		Severity:  domain.SeverityCritical,
		Active:    true,
	}
	err := svc.CreateRule(ctx, rule)
	if err != nil {
		t.Fatalf("unexpected error creating rule: %v", err)
	}

	alert := &domain.Alert{
		ID:           "alert-1",
		RuleID:       rule.ID,
		DeviceID:     "dev-123",
		Metric:       "temperature",
		TriggerValue: 85.5,
		Threshold:    80.0,
		Status:       domain.AlertStatusFiring,
		Message:      "High temp",
		FiredAt:      time.Now().UTC(),
	}
	_ = repo.CreateAlert(ctx, alert)

	err = svc.AcknowledgeAlert(ctx, "alert-1")
	if err != nil {
		t.Fatalf("unexpected error acknowledging alert: %v", err)
	}

	fetched, _ := svc.GetAlert(ctx, "alert-1")
	if fetched.Status != domain.AlertStatusAcknowledged {
		t.Fatalf("expected status ACKNOWLEDGED, got %v", fetched.Status)
	}

	err = svc.AcknowledgeAlert(ctx, "alert-1")
	if err != domain.ErrAlertAlreadyAcked {
		t.Fatalf("expected ErrAlertAlreadyAcked, got %v", err)
	}

	err = svc.ResolveAlert(ctx, "alert-1", "Temp back to normal")
	if err != nil {
		t.Fatalf("unexpected error resolving alert: %v", err)
	}

	fetched, _ = svc.GetAlert(ctx, "alert-1")
	if fetched.Status != domain.AlertStatusResolved {
		t.Fatalf("expected status RESOLVED, got %v", fetched.Status)
	}

	if len(repo.events) != 2 {
		t.Fatalf("expected 2 alert events, got %d", len(repo.events))
	}
}
