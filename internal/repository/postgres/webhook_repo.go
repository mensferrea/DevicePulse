package postgres

import (
	"context"
	"fmt"
	"time"

	"DevicePulse/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WebhookRepo struct {
	pool *pgxpool.Pool
}

func NewWebhookRepo(pool *pgxpool.Pool) *WebhookRepo {
	return &WebhookRepo{pool: pool}
}

func (r *WebhookRepo) CreateDelivery(ctx context.Context, d *domain.WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries (id, alert_id, alert_event_id, url, attempt, status, response_code, error_message, delivered_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	now := time.Now().UTC()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	var eventID *string
	if d.AlertEventID != "" {
		eventID = &d.AlertEventID
	}
	_, err := r.pool.Exec(ctx, query,
		d.ID, d.AlertID, eventID, d.URL, d.Attempt,
		string(d.Status), d.ResponseCode, d.ErrorMessage, d.DeliveredAt, d.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create webhook delivery: %w", err)
	}
	return nil
}

func (r *WebhookRepo) UpdateDeliveryStatus(ctx context.Context, id string, status domain.WebhookDeliveryStatus, code *int, errMsg *string, deliveredAt *time.Time) error {
	query := `
		UPDATE webhook_deliveries
		SET status = $2, response_code = $3, error_message = $4, delivered_at = $5
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, string(status), code, errMsg, deliveredAt)
	if err != nil {
		return fmt.Errorf("update webhook delivery status: %w", err)
	}
	return nil
}

func (r *WebhookRepo) ListDeliveries(ctx context.Context, alertID string) ([]domain.WebhookDelivery, error) {
	query := `
		SELECT id, alert_id, alert_event_id, url, attempt, status, response_code, error_message, delivered_at, created_at
		FROM webhook_deliveries
		WHERE alert_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, alertID)
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()

	var result []domain.WebhookDelivery
	for rows.Next() {
		var d domain.WebhookDelivery
		var eventID *string
		var st string
		if err := rows.Scan(
			&d.ID, &d.AlertID, &eventID, &d.URL, &d.Attempt,
			&st, &d.ResponseCode, &d.ErrorMessage, &d.DeliveredAt, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		if eventID != nil {
			d.AlertEventID = *eventID
		}
		d.Status = domain.WebhookDeliveryStatus(st)
		result = append(result, d)
	}
	return result, nil
}
