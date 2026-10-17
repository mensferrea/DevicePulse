package service

import (
	"context"
	"time"

	"DevicePulse/internal/domain"
	"github.com/google/uuid"
)

type MetricService struct {
	repo domain.MetricRepository
}

func NewMetricService(repo domain.MetricRepository) *MetricService {
	return &MetricService{repo: repo}
}

func (s *MetricService) CreateMetric(ctx context.Context, m *domain.MetricDefinition) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	return s.repo.Create(ctx, m)
}

func (s *MetricService) GetMetric(ctx context.Context, name string) (*domain.MetricDefinition, error) {
	return s.repo.GetByName(ctx, name)
}

func (s *MetricService) ListMetrics(ctx context.Context) ([]domain.MetricDefinition, error) {
	return s.repo.List(ctx)
}

func (s *MetricService) DeleteMetric(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
