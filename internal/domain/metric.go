package domain

import (
	"errors"
	"time"
)

type MetricDataType string

const (
	MetricTypeGauge   MetricDataType = "gauge"
	MetricTypeCounter MetricDataType = "counter"
)

type MetricDefinition struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	DataType    MetricDataType `json:"data_type"`
	Unit        string         `json:"unit"`
	MinValue    *float64       `json:"min_value,omitempty"`
	MaxValue    *float64       `json:"max_value,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

var (
	ErrMetricNotFound      = errors.New("metric definition not found")
	ErrMetricAlreadyExists = errors.New("metric definition already exists")
	ErrMetricValueOutOfRange = errors.New("measurement value out of allowed range")
)
