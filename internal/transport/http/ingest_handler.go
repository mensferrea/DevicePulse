package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"DevicePulse/internal/domain"
	"DevicePulse/internal/service"
)

type IngestHandler struct {
	ingestService *service.IngestService
}

func NewIngestHandler(is *service.IngestService) *IngestHandler {
	return &IngestHandler{ingestService: is}
}

type SingleMeasurementRequest struct {
	Metric     string    `json:"metric"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	RecordedAt time.Time `json:"recorded_at"`
}

type BatchTelemetryRequest struct {
	BatchID      string                     `json:"batch_id"`
	Measurements []SingleMeasurementRequest `json:"measurements"`
}

func (h *IngestHandler) IngestSingle(w http.ResponseWriter, r *http.Request) {
	deviceID := DeviceIDFromContext(r.Context())
	if deviceID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized device")
		return
	}

	var req SingleMeasurementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	m := domain.Measurement{
		DeviceID:   deviceID,
		Metric:     req.Metric,
		Value:      req.Value,
		Unit:       req.Unit,
		RecordedAt: req.RecordedAt,
	}

	err := h.ingestService.Ingest(r.Context(), deviceID, "", []domain.Measurement{m}, "rest")
	if err != nil {
		handleIngestError(w, err)
		return
	}

	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (h *IngestHandler) IngestBatch(w http.ResponseWriter, r *http.Request) {
	deviceID := DeviceIDFromContext(r.Context())
	if deviceID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized device")
		return
	}

	var req BatchTelemetryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	measurements := make([]domain.Measurement, len(req.Measurements))
	for i, item := range req.Measurements {
		measurements[i] = domain.Measurement{
			DeviceID:   deviceID,
			Metric:     item.Metric,
			Value:      item.Value,
			Unit:       item.Unit,
			RecordedAt: item.RecordedAt,
		}
	}

	err := h.ingestService.Ingest(r.Context(), deviceID, req.BatchID, measurements, "rest")
	if err != nil {
		handleIngestError(w, err)
		return
	}

	WriteJSON(w, http.StatusAccepted, map[string]any{
		"status":   "accepted",
		"accepted": len(measurements),
	})
}

func handleIngestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrEmptyBatch):
		WriteError(w, http.StatusBadRequest, "empty batch")
	case errors.Is(err, domain.ErrBatchTooLarge):
		WriteError(w, http.StatusRequestEntityTooLarge, "batch exceeds limit")
	case errors.Is(err, domain.ErrRateLimitExceeded):
		WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
	case errors.Is(err, domain.ErrFutureTimestamp):
		WriteError(w, http.StatusBadRequest, "timestamp too far in future")
	case errors.Is(err, domain.ErrTooOldTimestamp):
		WriteError(w, http.StatusBadRequest, "timestamp too old")
	case errors.Is(err, domain.ErrInvalidMeasurementData):
		WriteError(w, http.StatusBadRequest, "invalid measurement data")
	default:
		WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
