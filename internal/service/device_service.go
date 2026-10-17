package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"DevicePulse/internal/domain"
	"github.com/google/uuid"
)

type DeviceService struct {
	deviceRepo domain.DeviceRepository
	stateRepo  domain.StateRepository
}

func NewDeviceService(dr domain.DeviceRepository, sr domain.StateRepository) *DeviceService {
	return &DeviceService{
		deviceRepo: dr,
		stateRepo:  sr,
	}
}

func (s *DeviceService) RegisterDevice(ctx context.Context, name, group string) (*domain.Device, string, error) {
	deviceID := uuid.New().String()
	now := time.Now().UTC()

	device := &domain.Device{
		ID:        deviceID,
		Name:      name,
		Group:     group,
		Status:    domain.DeviceStatusUnknown,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.deviceRepo.Create(ctx, device); err != nil {
		return nil, "", err
	}

	apiKey, cred, err := s.generateCredential(deviceID)
	if err != nil {
		return nil, "", fmt.Errorf("generate credential: %w", err)
	}

	if err := s.deviceRepo.CreateCredential(ctx, cred); err != nil {
		return nil, "", fmt.Errorf("save credential: %w", err)
	}

	return device, apiKey, nil
}

func (s *DeviceService) IssueKey(ctx context.Context, deviceID string) (string, *domain.DeviceCredential, error) {
	if _, err := s.deviceRepo.GetByID(ctx, deviceID); err != nil {
		return "", nil, err
	}

	apiKey, cred, err := s.generateCredential(deviceID)
	if err != nil {
		return "", nil, err
	}

	if err := s.deviceRepo.CreateCredential(ctx, cred); err != nil {
		return "", nil, err
	}

	return apiKey, cred, nil
}

func (s *DeviceService) RevokeKey(ctx context.Context, credentialID string) error {
	return s.deviceRepo.RevokeCredential(ctx, credentialID)
}

func (s *DeviceService) Authenticate(ctx context.Context, apiKey string) (string, error) {
	if apiKey == "" {
		return "", domain.ErrInvalidCredentials
	}

	hash := hashKey(apiKey)
	cred, err := s.deviceRepo.GetCredentialByHash(ctx, hash)
	if err != nil {
		return "", domain.ErrInvalidCredentials
	}

	return cred.DeviceID, nil
}

func (s *DeviceService) GetDevice(ctx context.Context, id string) (*domain.Device, error) {
	return s.deviceRepo.GetByID(ctx, id)
}

func (s *DeviceService) ListDevices(ctx context.Context, filter domain.DeviceFilter) ([]domain.Device, error) {
	return s.deviceRepo.List(ctx, filter)
}

func (s *DeviceService) UpdateDevice(ctx context.Context, d *domain.Device) error {
	return s.deviceRepo.Update(ctx, d)
}

func (s *DeviceService) DeleteDevice(ctx context.Context, id string) error {
	return s.deviceRepo.Delete(ctx, id)
}

func (s *DeviceService) ListCredentials(ctx context.Context, deviceID string) ([]domain.DeviceCredential, error) {
	return s.deviceRepo.ListCredentials(ctx, deviceID)
}

func (s *DeviceService) CheckOfflineDevices(ctx context.Context, threshold time.Duration) (int, error) {
	candidates, err := s.deviceRepo.GetOfflineCandidates(ctx, threshold)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, dev := range candidates {
		err := s.deviceRepo.UpdateLastSeen(ctx, dev.ID, *dev.LastSeenAt, domain.DeviceStatusOffline)
		if err == nil {
			_ = s.stateRepo.SetDeviceStatus(ctx, dev.ID, domain.DeviceStatusOffline, *dev.LastSeenAt)
			updated++
		}
	}

	return updated, nil
}

func (s *DeviceService) generateCredential(deviceID string) (string, *domain.DeviceCredential, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", nil, err
	}
	rawToken := hex.EncodeToString(tokenBytes)
	apiKey := "dp_" + rawToken

	hash := hashKey(apiKey)
	prefix := apiKey[:11]

	cred := &domain.DeviceCredential{
		ID:        uuid.New().String(),
		DeviceID:  deviceID,
		KeyHash:   hash,
		KeyPrefix: prefix,
		Revoked:   false,
		IssuedAt:  time.Now().UTC(),
	}

	return apiKey, cred, nil
}

func hashKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}
