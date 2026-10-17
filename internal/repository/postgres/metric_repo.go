package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"DevicePulse/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MetricRepo struct {
	pool *pgxpool.Pool
}

func NewMetricRepo(pool *pgxpool.Pool) *MetricRepo {
	return &MetricRepo{pool: pool}
}

func (r *MetricRepo) Create(ctx context.Context, m *domain.MetricDefinition) error {
	query := `
		INSERT INTO metric_definitions (id, name, display_name, data_type, unit, min_value, max_value, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, query, m.ID, m.Name, m.DisplayName, string(m.DataType), m.Unit, m.MinValue, m.MaxValue, m.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrMetricAlreadyExists
		}
		return fmt.Errorf("create metric definition: %w", err)
	}
	return nil
}

func (r *MetricRepo) GetByName(ctx context.Context, name string) (*domain.MetricDefinition, error) {
	query := `
		SELECT id, name, display_name, data_type, unit, min_value, max_value, created_at
		FROM metric_definitions
		WHERE name = $1
	`
	var m domain.MetricDefinition
	var dt string
	err := r.pool.QueryRow(ctx, query, name).Scan(
		&m.ID, &m.Name, &m.DisplayName, &dt, &m.Unit, &m.MinValue, &m.MaxValue, &m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMetricNotFound
		}
		return nil, fmt.Errorf("get metric by name: %w", err)
	}
	m.DataType = domain.MetricDataType(dt)
	return &m, nil
}

func (r *MetricRepo) List(ctx context.Context) ([]domain.MetricDefinition, error) {
	query := `
		SELECT id, name, display_name, data_type, unit, min_value, max_value, created_at
		FROM metric_definitions
		ORDER BY name ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list metric definitions: %w", err)
	}
	defer rows.Close()

	var metrics []domain.MetricDefinition
	for rows.Next() {
		var m domain.MetricDefinition
		var dt string
		if err := rows.Scan(&m.ID, &m.Name, &m.DisplayName, &dt, &m.Unit, &m.MinValue, &m.MaxValue, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan metric definition: %w", err)
		}
		m.DataType = domain.MetricDataType(dt)
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (r *MetricRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM metric_definitions WHERE id = $1`
	cmd, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete metric: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrMetricNotFound
	}
	return nil
}
