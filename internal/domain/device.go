package domain

import (
	"errors"
	"time"
)

type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "ONLINE"
	DeviceStatusOffline DeviceStatus = "OFFLINE"
	DeviceStatusUnknown DeviceStatus = "UNKNOWN"
)

type Device struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Group      string       `json:"group"`
	Status     DeviceStatus `json:"status"`
	LastSeenAt *time.Time   `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

type DeviceCredential struct {
	ID        string     `json:"id"`
	DeviceID  string     `json:"device_id"`
	KeyHash   string     `json:"-"`
	KeyPrefix string     `json:"key_prefix"`
	Revoked   bool       `json:"revoked"`
	IssuedAt  time.Time  `json:"issued_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

var (
	ErrDeviceNotFound      = errors.New("device not found")
	ErrDeviceAlreadyExists = errors.New("device already exists")
	ErrInvalidCredentials  = errors.New("invalid or revoked device credentials")
	ErrDeviceKeyRevoked    = errors.New("device key revoked")
)
