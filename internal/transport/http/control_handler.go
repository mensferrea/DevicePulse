package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"DevicePulse/internal/domain"
	"DevicePulse/internal/service"
)

type ControlHandler struct {
	deviceService *service.DeviceService
	metricService *service.MetricService
	alertService  *service.AlertService
	telemetryRepo domain.TelemetryRepository
	stateRepo     domain.StateRepository
}

func NewControlHandler(
	ds *service.DeviceService,
	ms *service.MetricService,
	as *service.AlertService,
	tr domain.TelemetryRepository,
	sr domain.StateRepository,
) *ControlHandler {
	return &ControlHandler{
		deviceService: ds,
		metricService: ms,
		alertService:  as,
		telemetryRepo: tr,
		stateRepo:     sr,
	}
}

func (h *ControlHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Group string `json:"group"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "device name is required")
		return
	}

	device, apiKey, err := h.deviceService.RegisterDevice(r.Context(), req.Name, req.Group)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"device":  device,
		"api_key": apiKey,
	})
}

func (h *ControlHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	device, err := h.deviceService.GetDevice(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrDeviceNotFound) {
			WriteError(w, http.StatusNotFound, "device not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, device)
}

func (h *ControlHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	devices, err := h.deviceService.ListDevices(r.Context(), domain.DeviceFilter{
		Group:  group,
		Status: domain.DeviceStatus(status),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, devices)
}

func (h *ControlHandler) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name  string              `json:"name"`
		Group string              `json:"group"`
		Status domain.DeviceStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	dev := &domain.Device{
		ID:     id,
		Name:   req.Name,
		Group:  req.Group,
		Status: req.Status,
	}
	if err := h.deviceService.UpdateDevice(r.Context(), dev); err != nil {
		if errors.Is(err, domain.ErrDeviceNotFound) {
			WriteError(w, http.StatusNotFound, "device not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, dev)
}

func (h *ControlHandler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.deviceService.DeleteDevice(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrDeviceNotFound) {
			WriteError(w, http.StatusNotFound, "device not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ControlHandler) IssueKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	apiKey, cred, err := h.deviceService.IssueKey(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrDeviceNotFound) {
			WriteError(w, http.StatusNotFound, "device not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{
		"api_key":    apiKey,
		"credential": cred,
	})
}

func (h *ControlHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	keys, err := h.deviceService.ListCredentials(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, keys)
}

func (h *ControlHandler) RevokeKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.deviceService.RevokeKey(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrDeviceKeyRevoked) {
			WriteError(w, http.StatusNotFound, "key not found or already revoked")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ControlHandler) CreateMetric(w http.ResponseWriter, r *http.Request) {
	var m domain.MetricDefinition
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if m.Name == "" || m.Unit == "" {
		WriteError(w, http.StatusBadRequest, "metric name and unit are required")
		return
	}
	if err := h.metricService.CreateMetric(r.Context(), &m); err != nil {
		if errors.Is(err, domain.ErrMetricAlreadyExists) {
			WriteError(w, http.StatusConflict, "metric definition already exists")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, m)
}

func (h *ControlHandler) ListMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.metricService.ListMetrics(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, metrics)
}

func (h *ControlHandler) DeleteMetric(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.metricService.DeleteMetric(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrMetricNotFound) {
			WriteError(w, http.StatusNotFound, "metric not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ControlHandler) CreateAlertRule(w http.ResponseWriter, r *http.Request) {
	var rule domain.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if rule.Name == "" || rule.Metric == "" || rule.Operator == "" {
		WriteError(w, http.StatusBadRequest, "name, metric, and operator are required")
		return
	}
	if rule.Severity == "" {
		rule.Severity = domain.SeverityWarning
	}
	if err := h.alertService.CreateRule(r.Context(), &rule); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, rule)
}

func (h *ControlHandler) ListAlertRules(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Query().Get("metric")
	activeOnly := r.URL.Query().Get("active") == "true"
	rules, err := h.alertService.ListRules(r.Context(), metric, activeOnly)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, rules)
}

func (h *ControlHandler) GetAlertRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rule, err := h.alertService.GetRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrAlertRuleNotFound) {
			WriteError(w, http.StatusNotFound, "alert rule not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, rule)
}

func (h *ControlHandler) UpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var rule domain.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rule.ID = id
	if err := h.alertService.UpdateRule(r.Context(), &rule); err != nil {
		if errors.Is(err, domain.ErrAlertRuleNotFound) {
			WriteError(w, http.StatusNotFound, "alert rule not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, rule)
}

func (h *ControlHandler) DeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.alertService.DeleteRule(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrAlertRuleNotFound) {
			WriteError(w, http.StatusNotFound, "alert rule not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ControlHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	ruleID := r.URL.Query().Get("rule_id")
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	alerts, err := h.alertService.ListAlerts(r.Context(), domain.AlertFilter{
		DeviceID: deviceID,
		RuleID:   ruleID,
		Status:   domain.AlertStatus(status),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, alerts)
}

func (h *ControlHandler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.alertService.AcknowledgeAlert(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrAlertNotFound) {
			WriteError(w, http.StatusNotFound, "alert not found")
			return
		}
		if errors.Is(err, domain.ErrAlertAlreadyAcked) {
			WriteError(w, http.StatusConflict, "alert already acknowledged")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

func (h *ControlHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.alertService.ResolveAlert(r.Context(), id, req.Reason); err != nil {
		if errors.Is(err, domain.ErrAlertNotFound) {
			WriteError(w, http.StatusNotFound, "alert not found")
			return
		}
		if errors.Is(err, domain.ErrAlertResolved) {
			WriteError(w, http.StatusConflict, "alert already resolved")
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (h *ControlHandler) GetDeviceLatestState(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	state, err := h.stateRepo.GetLatestState(r.Context(), deviceID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if state.LastSeenAt == nil {
		dev, err := h.deviceService.GetDevice(r.Context(), deviceID)
		if err == nil {
			state.Status = dev.Status
			state.LastSeenAt = dev.LastSeenAt
		}
	}

	WriteJSON(w, http.StatusOK, state)
}

func (h *ControlHandler) GetDeviceMeasurements(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	metric := r.URL.Query().Get("metric")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	var from, to time.Time
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		from, _ = time.Parse(time.RFC3339, fromStr)
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		to, _ = time.Parse(time.RFC3339, toStr)
	}

	measurements, err := h.telemetryRepo.GetMeasurements(r.Context(), domain.MeasurementFilter{
		DeviceID: deviceID,
		Metric:   metric,
		From:     from,
		To:       to,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, measurements)
}

func (h *ControlHandler) GetDeviceStats(w http.ResponseWriter, r *http.Request) {
	deviceID := r.PathValue("id")
	metric := r.URL.Query().Get("metric")
	interval := r.URL.Query().Get("interval")

	var from, to time.Time
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		from, _ = time.Parse(time.RFC3339, fromStr)
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		to, _ = time.Parse(time.RFC3339, toStr)
	}

	if interval != "" {
		aggregates, err := h.telemetryRepo.GetAggregated(r.Context(), domain.AggregationFilter{
			DeviceID: deviceID,
			Metric:   metric,
			Interval: interval,
			From:     from,
			To:       to,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		WriteJSON(w, http.StatusOK, aggregates)
		return
	}

	stats, err := h.telemetryRepo.GetStats(r.Context(), deviceID, metric, from, to)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, stats)
}
