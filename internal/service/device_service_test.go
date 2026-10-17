package service_test

import (
	"context"
	"testing"
	"time"

	"DevicePulse/internal/domain"
	"DevicePulse/internal/service"
)

type mockDeviceRepo struct {
	devices     map[string]*domain.Device
	credentials map[string]*domain.DeviceCredential
}

func newMockDeviceRepo() *mockDeviceRepo {
	return &mockDeviceRepo{
		devices:     make(map[string]*domain.Device),
		credentials: make(map[string]*domain.DeviceCredential),
	}
}

func (m *mockDeviceRepo) Create(ctx context.Context, device *domain.Device) error {
	m.devices[device.ID] = device
	return nil
}

func (m *mockDeviceRepo) GetByID(ctx context.Context, id string) (*domain.Device, error) {
	d, ok := m.devices[id]
	if !ok {
		return nil, domain.ErrDeviceNotFound
	}
	return d, nil
}

func (m *mockDeviceRepo) List(ctx context.Context, filter domain.DeviceFilter) ([]domain.Device, error) {
	var res []domain.Device
	for _, d := range m.devices {
		res = append(res, *d)
	}
	return res, nil
}

func (m *mockDeviceRepo) Update(ctx context.Context, device *domain.Device) error {
	m.devices[device.ID] = device
	return nil
}

func (m *mockDeviceRepo) Delete(ctx context.Context, id string) error {
	delete(m.devices, id)
	return nil
}

func (m *mockDeviceRepo) UpdateLastSeen(ctx context.Context, deviceID string, lastSeenAt time.Time, status domain.DeviceStatus) error {
	d, ok := m.devices[deviceID]
	if !ok {
		return domain.ErrDeviceNotFound
	}
	d.LastSeenAt = &lastSeenAt
	d.Status = status
	return nil
}

func (m *mockDeviceRepo) CreateCredential(ctx context.Context, cred *domain.DeviceCredential) error {
	m.credentials[cred.KeyHash] = cred
	return nil
}

func (m *mockDeviceRepo) GetCredentialByHash(ctx context.Context, keyHash string) (*domain.DeviceCredential, error) {
	cred, ok := m.credentials[keyHash]
	if !ok || cred.Revoked {
		return nil, domain.ErrInvalidCredentials
	}
	return cred, nil
}

func (m *mockDeviceRepo) ListCredentials(ctx context.Context, deviceID string) ([]domain.DeviceCredential, error) {
	var res []domain.DeviceCredential
	for _, c := range m.credentials {
		if c.DeviceID == deviceID {
			res = append(res, *c)
		}
	}
	return res, nil
}

func (m *mockDeviceRepo) RevokeCredential(ctx context.Context, credentialID string) error {
	for _, c := range m.credentials {
		if c.ID == credentialID {
			c.Revoked = true
			return nil
		}
	}
	return domain.ErrDeviceKeyRevoked
}

func (m *mockDeviceRepo) GetOfflineCandidates(ctx context.Context, threshold time.Duration) ([]domain.Device, error) {
	cutoff := time.Now().UTC().Add(-threshold)
	var res []domain.Device
	for _, d := range m.devices {
		if d.Status == domain.DeviceStatusOnline && (d.LastSeenAt == nil || d.LastSeenAt.Before(cutoff)) {
			res = append(res, *d)
		}
	}
	return res, nil
}

type mockStateRepo struct {
	statuses map[string]domain.DeviceStatus
}

func newMockStateRepo() *mockStateRepo {
	return &mockStateRepo{statuses: make(map[string]domain.DeviceStatus)}
}

func (m *mockStateRepo) SetLatestPoint(ctx context.Context, deviceID, metric string, point domain.LatestPoint) error {
	return nil
}
func (m *mockStateRepo) GetLatestState(ctx context.Context, deviceID string) (*domain.DeviceLatestState, error) {
	return nil, nil
}
func (m *mockStateRepo) SetDeviceStatus(ctx context.Context, deviceID string, status domain.DeviceStatus, lastSeen time.Time) error {
	m.statuses[deviceID] = status
	return nil
}
func (m *mockStateRepo) GetDeviceStatus(ctx context.Context, deviceID string) (domain.DeviceStatus, *time.Time, error) {
	st, ok := m.statuses[deviceID]
	if !ok {
		return domain.DeviceStatusUnknown, nil, nil
	}
	return st, nil, nil
}
func (m *mockStateRepo) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return true, nil
}
func (m *mockStateRepo) IsMeasurementSeen(ctx context.Context, deviceID, metric string, recordedAt time.Time, ttl time.Duration) (bool, error) {
	return false, nil
}

func TestDeviceRegistrationAndAuthentication(t *testing.T) {
	ctx := context.Background()
	devRepo := newMockDeviceRepo()
	stRepo := newMockStateRepo()
	svc := service.NewDeviceService(devRepo, stRepo)

	device, apiKey, err := svc.RegisterDevice(ctx, "weather-station-1", "sensors")
	if err != nil {
		t.Fatalf("unexpected error registering device: %v", err)
	}

	if device.ID == "" || apiKey == "" {
		t.Fatalf("device ID or API key is empty")
	}

	authDeviceID, err := svc.Authenticate(ctx, apiKey)
	if err != nil {
		t.Fatalf("authentication failed with valid key: %v", err)
	}
	if authDeviceID != device.ID {
		t.Fatalf("expected device ID %s, got %s", device.ID, authDeviceID)
	}

	_, err = svc.Authenticate(ctx, "invalid_key_123")
	if err != domain.ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestDeviceOfflineDetection(t *testing.T) {
	ctx := context.Background()
	devRepo := newMockDeviceRepo()
	stRepo := newMockStateRepo()
	svc := service.NewDeviceService(devRepo, stRepo)

	oldTime := time.Now().UTC().Add(-10 * time.Minute)
	dev := &domain.Device{
		ID:         "dev-offline-test",
		Name:       "offline-sensor",
		Status:     domain.DeviceStatusOnline,
		LastSeenAt: &oldTime,
	}
	_ = devRepo.Create(ctx, dev)

	count, err := svc.CheckOfflineDevices(ctx, 1*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error checking offline devices: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 offline device detected, got %d", count)
	}

	updated, _ := devRepo.GetByID(ctx, "dev-offline-test")
	if updated.Status != domain.DeviceStatusOffline {
		t.Fatalf("expected status OFFLINE, got %s", updated.Status)
	}
}
