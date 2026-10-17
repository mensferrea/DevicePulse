package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"DevicePulse/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AlertRepo struct {
	pool *pgxpool.Pool
}

func NewAlertRepo(pool *pgxpool.Pool) *AlertRepo {
	return &AlertRepo{pool: pool}
}

func (r *AlertRepo) CreateRule(ctx context.Context, rule *domain.AlertRule) error {
	query := `
		INSERT INTO alert_rules (id, name, device_id, group_name, metric, operator, threshold, duration_seconds, severity, active, webhook_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	now := time.Now().UTC()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	rule.UpdatedAt = now
	_, err := r.pool.Exec(ctx, query,
		rule.ID, rule.Name, rule.DeviceID, rule.GroupName, rule.Metric,
		string(rule.Operator), rule.Threshold, rule.DurationSeconds,
		string(rule.Severity), rule.Active, rule.WebhookURL, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create alert rule: %w", err)
	}
	return nil
}

func (r *AlertRepo) GetRuleByID(ctx context.Context, id string) (*domain.AlertRule, error) {
	query := `
		SELECT id, name, device_id, group_name, metric, operator, threshold, duration_seconds, severity, active, webhook_url, created_at, updated_at
		FROM alert_rules
		WHERE id = $1
	`
	var rule domain.AlertRule
	var op, sev string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&rule.ID, &rule.Name, &rule.DeviceID, &rule.GroupName, &rule.Metric,
		&op, &rule.Threshold, &rule.DurationSeconds, &sev, &rule.Active,
		&rule.WebhookURL, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAlertRuleNotFound
		}
		return nil, fmt.Errorf("get alert rule: %w", err)
	}
	rule.Operator = domain.AlertOperator(op)
	rule.Severity = domain.AlertSeverity(sev)
	return &rule, nil
}

func (r *AlertRepo) ListRules(ctx context.Context, metric string, activeOnly bool) ([]domain.AlertRule, error) {
	query := `
		SELECT id, name, device_id, group_name, metric, operator, threshold, duration_seconds, severity, active, webhook_url, created_at, updated_at
		FROM alert_rules
		WHERE ($1 = '' OR metric = $1)
		  AND ($2 = FALSE OR active = TRUE)
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, metric, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()

	var rules []domain.AlertRule
	for rows.Next() {
		var rule domain.AlertRule
		var op, sev string
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.DeviceID, &rule.GroupName, &rule.Metric,
			&op, &rule.Threshold, &rule.DurationSeconds, &sev, &rule.Active,
			&rule.WebhookURL, &rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		rule.Operator = domain.AlertOperator(op)
		rule.Severity = domain.AlertSeverity(sev)
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *AlertRepo) UpdateRule(ctx context.Context, rule *domain.AlertRule) error {
	query := `
		UPDATE alert_rules
		SET name = $2, device_id = $3, group_name = $4, metric = $5, operator = $6,
		    threshold = $7, duration_seconds = $8, severity = $9, active = $10,
		    webhook_url = $11, updated_at = $12
		WHERE id = $1
	`
	rule.UpdatedAt = time.Now().UTC()
	cmd, err := r.pool.Exec(ctx, query,
		rule.ID, rule.Name, rule.DeviceID, rule.GroupName, rule.Metric,
		string(rule.Operator), rule.Threshold, rule.DurationSeconds,
		string(rule.Severity), rule.Active, rule.WebhookURL, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update alert rule: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrAlertRuleNotFound
	}
	return nil
}

func (r *AlertRepo) DeleteRule(ctx context.Context, id string) error {
	query := `DELETE FROM alert_rules WHERE id = $1`
	cmd, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrAlertRuleNotFound
	}
	return nil
}

func (r *AlertRepo) GetActiveAlert(ctx context.Context, deviceID, ruleID string) (*domain.Alert, error) {
	query := `
		SELECT id, rule_id, device_id, metric, trigger_value, threshold, status, message, fired_at, acknowledged_at, resolved_at, created_at, updated_at
		FROM alerts
		WHERE device_id = $1 AND rule_id = $2 AND status IN ('PENDING', 'FIRING', 'ACKNOWLEDGED')
		ORDER BY created_at DESC
		LIMIT 1
	`
	var a domain.Alert
	var st string
	err := r.pool.QueryRow(ctx, query, deviceID, ruleID).Scan(
		&a.ID, &a.RuleID, &a.DeviceID, &a.Metric, &a.TriggerValue, &a.Threshold,
		&st, &a.Message, &a.FiredAt, &a.AcknowledgedAt, &a.ResolvedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active alert: %w", err)
	}
	a.Status = domain.AlertStatus(st)
	return &a, nil
}

func (r *AlertRepo) CreateAlert(ctx context.Context, a *domain.Alert) error {
	query := `
		INSERT INTO alerts (id, rule_id, device_id, metric, trigger_value, threshold, status, message, fired_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.FiredAt.IsZero() {
		a.FiredAt = now
	}
	a.UpdatedAt = now
	_, err := r.pool.Exec(ctx, query,
		a.ID, a.RuleID, a.DeviceID, a.Metric, a.TriggerValue, a.Threshold,
		string(a.Status), a.Message, a.FiredAt, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create alert: %w", err)
	}
	return nil
}

func (r *AlertRepo) UpdateAlertStatus(ctx context.Context, alertID string, status domain.AlertStatus, ackAt, resAt *time.Time) error {
	query := `
		UPDATE alerts
		SET status = $2, acknowledged_at = $3, resolved_at = $4, updated_at = NOW()
		WHERE id = $1
	`
	cmd, err := r.pool.Exec(ctx, query, alertID, string(status), ackAt, resAt)
	if err != nil {
		return fmt.Errorf("update alert status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrAlertNotFound
	}
	return nil
}

func (r *AlertRepo) CreateEvent(ctx context.Context, ev *domain.AlertEvent) error {
	query := `
		INSERT INTO alert_events (id, alert_id, old_status, new_status, trigger_value, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, query, ev.ID, ev.AlertID, string(ev.OldStatus), string(ev.NewStatus), ev.TriggerValue, ev.Reason, ev.CreatedAt)
	if err != nil {
		return fmt.Errorf("create alert event: %w", err)
	}
	return nil
}

func (r *AlertRepo) ListAlerts(ctx context.Context, filter domain.AlertFilter) ([]domain.Alert, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, rule_id, device_id, metric, trigger_value, threshold, status, message, fired_at, acknowledged_at, resolved_at, created_at, updated_at
		FROM alerts
		WHERE ($1 = '' OR device_id = $1)
		  AND ($2 = '' OR rule_id = $2::uuid)
		  AND ($3 = '' OR status = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`
	var ruleIDParam *string
	if filter.RuleID != "" {
		ruleIDParam = &filter.RuleID
	}

	rows, err := r.pool.Query(ctx, query, filter.DeviceID, ruleIDParam, string(filter.Status), limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var alerts []domain.Alert
	for rows.Next() {
		var a domain.Alert
		var st string
		if err := rows.Scan(
			&a.ID, &a.RuleID, &a.DeviceID, &a.Metric, &a.TriggerValue, &a.Threshold,
			&st, &a.Message, &a.FiredAt, &a.AcknowledgedAt, &a.ResolvedAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		a.Status = domain.AlertStatus(st)
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (r *AlertRepo) GetAlertByID(ctx context.Context, id string) (*domain.Alert, error) {
	query := `
		SELECT id, rule_id, device_id, metric, trigger_value, threshold, status, message, fired_at, acknowledged_at, resolved_at, created_at, updated_at
		FROM alerts
		WHERE id = $1
	`
	var a domain.Alert
	var st string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.RuleID, &a.DeviceID, &a.Metric, &a.TriggerValue, &a.Threshold,
		&st, &a.Message, &a.FiredAt, &a.AcknowledgedAt, &a.ResolvedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAlertNotFound
		}
		return nil, fmt.Errorf("get alert by id: %w", err)
	}
	a.Status = domain.AlertStatus(st)
	return &a, nil
}
