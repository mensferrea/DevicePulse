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

type DeviceRepo struct {
	pool *pgxpool.Pool
}

func NewDeviceRepo(pool *pgxpool.Pool) *DeviceRepo {
	return &DeviceRepo{pool: pool}
}

func (r *DeviceRepo) Create(ctx context.Context, d *domain.Device) error {
	query := `
		INSERT INTO devices (id, name, group_name, status, last_seen_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	now := time.Now().UTC()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	d.UpdatedAt = now
	if d.Status == "" {
		d.Status = domain.DeviceStatusUnknown
	}
	if d.Group == "" {
		d.Group = "default"
	}

	_, err := r.pool.Exec(ctx, query, d.ID, d.Name, d.Group, d.Status, d.LastSeenAt, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDeviceAlreadyExists
		}
		return fmt.Errorf("create device: %w", err)
	}
	return nil
}

func (r *DeviceRepo) GetByID(ctx context.Context, id string) (*domain.Device, error) {
	query := `
		SELECT id, name, group_name, status, last_seen_at, created_at, updated_at
		FROM devices
		WHERE id = $1
	`
	var d domain.Device
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&d.ID, &d.Name, &d.Group, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDeviceNotFound
		}
		return nil, fmt.Errorf("get device by id: %w", err)
	}
	return &d, nil
}

func (r *DeviceRepo) List(ctx context.Context, filter domain.DeviceFilter) ([]domain.Device, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT id, name, group_name, status, last_seen_at, created_at, updated_at
		FROM devices
		WHERE ($1 = '' OR group_name = $1)
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.pool.Query(ctx, query, filter.Group, string(filter.Status), limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	var devices []domain.Device
	for rows.Next() {
		var d domain.Device
		if err := rows.Scan(&d.ID, &d.Name, &d.Group, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func (r *DeviceRepo) Update(ctx context.Context, d *domain.Device) error {
	query := `
		UPDATE devices
		SET name = $2, group_name = $3, status = $4, updated_at = $5
		WHERE id = $1
	`
	d.UpdatedAt = time.Now().UTC()
	cmd, err := r.pool.Exec(ctx, query, d.ID, d.Name, d.Group, d.Status, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update device: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrDeviceNotFound
	}
	return nil
}

func (r *DeviceRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM devices WHERE id = $1`
	cmd, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrDeviceNotFound
	}
	return nil
}

func (r *DeviceRepo) UpdateLastSeen(ctx context.Context, deviceID string, lastSeenAt time.Time, status domain.DeviceStatus) error {
	query := `
		UPDATE devices
		SET last_seen_at = $2, status = $3, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, deviceID, lastSeenAt, status)
	if err != nil {
		return fmt.Errorf("update last seen: %w", err)
	}
	return nil
}

func (r *DeviceRepo) CreateCredential(ctx context.Context, cred *domain.DeviceCredential) error {
	query := `
		INSERT INTO device_credentials (id, device_id, key_hash, key_prefix, revoked, issued_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if cred.IssuedAt.IsZero() {
		cred.IssuedAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, query, cred.ID, cred.DeviceID, cred.KeyHash, cred.KeyPrefix, cred.Revoked, cred.IssuedAt)
	if err != nil {
		return fmt.Errorf("create device credential: %w", err)
	}
	return nil
}

func (r *DeviceRepo) GetCredentialByHash(ctx context.Context, keyHash string) (*domain.DeviceCredential, error) {
	query := `
		SELECT id, device_id, key_hash, key_prefix, revoked, issued_at, revoked_at
		FROM device_credentials
		WHERE key_hash = $1 AND revoked = FALSE
	`
	var c domain.DeviceCredential
	err := r.pool.QueryRow(ctx, query, keyHash).Scan(
		&c.ID, &c.DeviceID, &c.KeyHash, &c.KeyPrefix, &c.Revoked, &c.IssuedAt, &c.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get credential by hash: %w", err)
	}
	return &c, nil
}

func (r *DeviceRepo) ListCredentials(ctx context.Context, deviceID string) ([]domain.DeviceCredential, error) {
	query := `
		SELECT id, device_id, key_prefix, revoked, issued_at, revoked_at
		FROM device_credentials
		WHERE device_id = $1
		ORDER BY issued_at DESC
	`
	rows, err := r.pool.Query(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()

	var creds []domain.DeviceCredential
	for rows.Next() {
		var c domain.DeviceCredential
		if err := rows.Scan(&c.ID, &c.DeviceID, &c.KeyPrefix, &c.Revoked, &c.IssuedAt, &c.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		creds = append(creds, c)
	}
	return creds, nil
}

func (r *DeviceRepo) RevokeCredential(ctx context.Context, credentialID string) error {
	query := `
		UPDATE device_credentials
		SET revoked = TRUE, revoked_at = NOW()
		WHERE id = $1 AND revoked = FALSE
	`
	cmd, err := r.pool.Exec(ctx, query, credentialID)
	if err != nil {
		return fmt.Errorf("revoke credential: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrDeviceKeyRevoked
	}
	return nil
}

func (r *DeviceRepo) GetOfflineCandidates(ctx context.Context, threshold time.Duration) ([]domain.Device, error) {
	cutoff := time.Now().UTC().Add(-threshold)
	query := `
		SELECT id, name, group_name, status, last_seen_at, created_at, updated_at
		FROM devices
		WHERE status = 'ONLINE' AND (last_seen_at IS NULL OR last_seen_at < $1)
	`
	rows, err := r.pool.Query(ctx, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("get offline candidates: %w", err)
	}
	defer rows.Close()

	var devices []domain.Device
	for rows.Next() {
		var d domain.Device
		if err := rows.Scan(&d.ID, &d.Name, &d.Group, &d.Status, &d.LastSeenAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan offline device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}
