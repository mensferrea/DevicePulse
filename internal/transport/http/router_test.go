package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"DevicePulse/internal/domain"
	"DevicePulse/internal/service"
	httptransport "DevicePulse/internal/transport/http"
)

type mockDeviceRepo struct {
	devices     map[string]*domain.Device
	credentials map[string]*domain.DeviceCredential
}

func (m *mockDeviceRepo) Create(ctx context.Context, d *domain.Device) error {
	m.devices[d.ID] = d
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
	var list []domain.Device
	for _, d := range m.devices {
		list = append(list, *d)
	}
	return list, nil
}
func (m *mockDeviceRepo) Update(ctx context.Context, d *domain.Device) error {
	m.devices[d.ID] = d
	return nil
}
func (m *mockDeviceRepo) Delete(ctx context.Context, id string) error {
	delete(m.devices, id)
	return nil
}
func (m *mockDeviceRepo) UpdateLastSeen(ctx context.Context, deviceID string, lastSeenAt time.Time, status domain.DeviceStatus) error {
	return nil
}
func (m *mockDeviceRepo) CreateCredential(ctx context.Context, cred *domain.DeviceCredential) error {
	m.credentials[cred.KeyHash] = cred
	return nil
}
func (m *mockDeviceRepo) GetCredentialByHash(ctx context.Context, keyHash string) (*domain.DeviceCredential, error) {
	c, ok := m.credentials[keyHash]
	if !ok || c.Revoked {
		return nil, domain.ErrInvalidCredentials
	}
	return c, nil
}
func (m *mockDeviceRepo) ListCredentials(ctx context.Context, deviceID string) ([]domain.DeviceCredential, error) {
	return nil, nil
}
func (m *mockDeviceRepo) RevokeCredential(ctx context.Context, credentialID string) error {
	return nil
}
func (m *mockDeviceRepo) GetOfflineCandidates(ctx context.Context, threshold time.Duration) ([]domain.Device, error) {
	return nil, nil
}

type mockStateRepo struct{}

func (m *mockStateRepo) SetLatestPoint(ctx context.Context, deviceID, metric string, point domain.LatestPoint) error {
	return nil
}
func (m *mockStateRepo) GetLatestState(ctx context.Context, deviceID string) (*domain.DeviceLatestState, error) {
	return &domain.DeviceLatestState{
		DeviceID: deviceID,
		Status:   domain.DeviceStatusOnline,
		Metrics:  make(map[string]domain.LatestPoint),
	}, nil
}
func (m *mockStateRepo) SetDeviceStatus(ctx context.Context, deviceID string, status domain.DeviceStatus, lastSeen time.Time) error {
	return nil
}
func (m *mockStateRepo) GetDeviceStatus(ctx context.Context, deviceID string) (domain.DeviceStatus, *time.Time, error) {
	return domain.DeviceStatusOnline, nil, nil
}
func (m *mockStateRepo) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return true, nil
}
func (m *mockStateRepo) IsMeasurementSeen(ctx context.Context, deviceID, metric string, recordedAt time.Time, ttl time.Duration) (bool, error) {
	return false, nil
}

func TestHealthCheck(t *testing.T) {
	router := httptransport.NewRouter(httptransport.RouterConfig{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var res map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res["status"] != "ok" {
		t.Fatalf("expected status 'ok', got '%s'", res["status"])
	}
}

func TestDeviceRegistrationEndpoint(t *testing.T) {
	devRepo := &mockDeviceRepo{
		devices:     make(map[string]*domain.Device),
		credentials: make(map[string]*domain.DeviceCredential),
	}
	stateRepo := &mockStateRepo{}
	devSvc := service.NewDeviceService(devRepo, stateRepo)
	ctrlHandler := httptransport.NewControlHandler(devSvc, nil, nil, nil, stateRepo)

	router := httptransport.NewRouter(httptransport.RouterConfig{
		ControlHandler: ctrlHandler,
		DeviceService:  devSvc,
	})

	body, _ := json.Marshal(map[string]string{
		"name":  "test-device",
		"group": "sensors",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var res struct {
		Device domain.Device `json:"device"`
		APIKey string        `json:"api_key"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Device.Name != "test-device" || res.APIKey == "" {
		t.Fatalf("invalid device response: %+v", res)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/devices/"+res.Device.ID, nil)
	getRR := httptest.NewRecorder()
	router.ServeHTTP(getRR, getReq)

	if getRR.Code != http.StatusOK {
		t.Fatalf("expected status 200 on get device, got %d", getRR.Code)
	}
}
