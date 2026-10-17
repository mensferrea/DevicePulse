package postgres

import (
	"context"
	"fmt"
	"time"

	"DevicePulse/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TelemetryRepo struct {
	pool *pgxpool.Pool
}

func NewTelemetryRepo(pool *pgxpool.Pool) *TelemetryRepo {
	return &TelemetryRepo{pool: pool}
}

func (r *TelemetryRepo) CreateBatchRecord(ctx context.Context, b *domain.TelemetryBatch) error {
	query := `
		INSERT INTO telemetry_batches (id, device_id, batch_id, status, received_count, processed_count, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (device_id, batch_id) DO NOTHING
	`
	if b.ReceivedAt.IsZero() {
		b.ReceivedAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, query, b.ID, b.DeviceID, b.BatchID, b.Status, b.ReceivedCount, b.ProcessedCount, b.ReceivedAt)
	if err != nil {
		return fmt.Errorf("create batch record: %w", err)
	}
	return nil
}

func (r *TelemetryRepo) UpdateBatchStatus(ctx context.Context, deviceID, batchID, status string, processedCount int) error {
	query := `
		UPDATE telemetry_batches
		SET status = $3, processed_count = $4, processed_at = NOW()
		WHERE device_id = $1 AND batch_id = $2
	`
	_, err := r.pool.Exec(ctx, query, deviceID, batchID, status, processedCount)
	if err != nil {
		return fmt.Errorf("update batch status: %w", err)
	}
	return nil
}

func (r *TelemetryRepo) InsertMeasurements(ctx context.Context, measurements []domain.Measurement) (int, error) {
	if len(measurements) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	query := `
		INSERT INTO measurements (device_id, metric, value, unit, recorded_at, ingested_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (device_id, metric, recorded_at) DO NOTHING
	`

	now := time.Now().UTC()
	for _, m := range measurements {
		ingestedAt := m.IngestedAt
		if ingestedAt.IsZero() {
			ingestedAt = now
		}
		batch.Queue(query, m.DeviceID, m.Metric, m.Value, m.Unit, m.RecordedAt, ingestedAt)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	insertedCount := 0
	for i := 0; i < len(measurements); i++ {
		cmd, err := br.Exec()
		if err != nil {
			return insertedCount, fmt.Errorf("exec batch item %d: %w", i, err)
		}
		if cmd.RowsAffected() > 0 {
			insertedCount++
		}
	}

	return insertedCount, nil
}

func (r *TelemetryRepo) GetMeasurements(ctx context.Context, filter domain.MeasurementFilter) ([]domain.Measurement, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
		SELECT id, device_id, metric, value, unit, recorded_at, ingested_at
		FROM measurements
		WHERE device_id = $1
		  AND ($2 = '' OR metric = $2)
		  AND ($3::timestamptz IS NULL OR recorded_at >= $3)
		  AND ($4::timestamptz IS NULL OR recorded_at <= $4)
		ORDER BY recorded_at DESC
		LIMIT $5 OFFSET $6
	`

	var fromPtr, toPtr *time.Time
	if !filter.From.IsZero() {
		fromPtr = &filter.From
	}
	if !filter.To.IsZero() {
		toPtr = &filter.To
	}

	rows, err := r.pool.Query(ctx, query, filter.DeviceID, filter.Metric, fromPtr, toPtr, limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("get measurements: %w", err)
	}
	defer rows.Close()

	var result []domain.Measurement
	for rows.Next() {
		var m domain.Measurement
		if err := rows.Scan(&m.ID, &m.DeviceID, &m.Metric, &m.Value, &m.Unit, &m.RecordedAt, &m.IngestedAt); err != nil {
			return nil, fmt.Errorf("scan measurement: %w", err)
		}
		result = append(result, m)
	}

	return result, nil
}

func (r *TelemetryRepo) GetAggregated(ctx context.Context, filter domain.AggregationFilter) ([]domain.AggregatedMeasurement, error) {
	intervalStr := "1 minute"
	if filter.Interval == "5m" || filter.Interval == "5 minute" {
		intervalStr = "5 minutes"
	} else if filter.Interval == "1h" || filter.Interval == "1 hour" {
		intervalStr = "1 hour"
	}

	query := fmt.Sprintf(`
		SELECT 
			device_id,
			metric,
			date_bin(INTERVAL '%s', recorded_at, TIMESTAMP '2000-01-01') AS bucket,
			MIN(value) AS min_val,
			MAX(value) AS max_val,
			AVG(value) AS avg_val,
			COUNT(*) AS count_val
		FROM measurements
		WHERE device_id = $1
		  AND ($2 = '' OR metric = $2)
		  AND ($3::timestamptz IS NULL OR recorded_at >= $3)
		  AND ($4::timestamptz IS NULL OR recorded_at <= $4)
		GROUP BY device_id, metric, bucket
		ORDER BY bucket ASC
	`, intervalStr)

	var fromPtr, toPtr *time.Time
	if !filter.From.IsZero() {
		fromPtr = &filter.From
	}
	if !filter.To.IsZero() {
		toPtr = &filter.To
	}

	rows, err := r.pool.Query(ctx, query, filter.DeviceID, filter.Metric, fromPtr, toPtr)
	if err != nil {
		return nil, fmt.Errorf("get aggregated measurements: %w", err)
	}
	defer rows.Close()

	var results []domain.AggregatedMeasurement
	for rows.Next() {
		var a domain.AggregatedMeasurement
		if err := rows.Scan(&a.DeviceID, &a.Metric, &a.Bucket, &a.Min, &a.Max, &a.Avg, &a.Count); err != nil {
			return nil, fmt.Errorf("scan aggregated measurement: %w", err)
		}
		results = append(results, a)
	}

	return results, nil
}

func (r *TelemetryRepo) GetStats(ctx context.Context, deviceID, metric string, from, to time.Time) (*domain.TelemetryStats, error) {
	query := `
		SELECT 
			COALESCE(MIN(value), 0),
			COALESCE(MAX(value), 0),
			COALESCE(AVG(value), 0),
			COUNT(*)
		FROM measurements
		WHERE device_id = $1 AND metric = $2
		  AND ($3::timestamptz IS NULL OR recorded_at >= $3)
		  AND ($4::timestamptz IS NULL OR recorded_at <= $4)
	`
	var fromPtr, toPtr *time.Time
	if !from.IsZero() {
		fromPtr = &from
	}
	if !to.IsZero() {
		toPtr = &to
	}

	var stats domain.TelemetryStats
	stats.Metric = metric
	err := r.pool.QueryRow(ctx, query, deviceID, metric, fromPtr, toPtr).Scan(
		&stats.Min, &stats.Max, &stats.Avg, &stats.Count,
	)
	if err != nil {
		return nil, fmt.Errorf("get telemetry stats: %w", err)
	}

	return &stats, nil
}
