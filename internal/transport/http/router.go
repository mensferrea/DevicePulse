package http

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/pprof"

	"DevicePulse/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed static/*
var staticFS embed.FS

type RouterConfig struct {
	ControlHandler *ControlHandler
	IngestHandler  *IngestHandler
	DeviceService  *service.DeviceService
}

func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.Handle("GET /metrics", promhttp.Handler())

	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

	staticSub, err := fs.Sub(staticFS, "static")
	if err == nil {
		fileServer := http.FileServer(http.FS(staticSub))
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
		mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))
	}

	if cfg.ControlHandler != nil {
		ch := cfg.ControlHandler
		mux.HandleFunc("POST /api/v1/devices", ch.RegisterDevice)
		mux.HandleFunc("GET /api/v1/devices", ch.ListDevices)
		mux.HandleFunc("GET /api/v1/devices/{id}", ch.GetDevice)
		mux.HandleFunc("PUT /api/v1/devices/{id}", ch.UpdateDevice)
		mux.HandleFunc("DELETE /api/v1/devices/{id}", ch.DeleteDevice)
		mux.HandleFunc("POST /api/v1/devices/{id}/keys", ch.IssueKey)
		mux.HandleFunc("GET /api/v1/devices/{id}/keys", ch.ListKeys)
		mux.HandleFunc("DELETE /api/v1/credentials/{id}", ch.RevokeKey)

		mux.HandleFunc("POST /api/v1/metrics", ch.CreateMetric)
		mux.HandleFunc("GET /api/v1/metrics", ch.ListMetrics)
		mux.HandleFunc("DELETE /api/v1/metrics/{id}", ch.DeleteMetric)

		mux.HandleFunc("POST /api/v1/alert-rules", ch.CreateAlertRule)
		mux.HandleFunc("GET /api/v1/alert-rules", ch.ListAlertRules)
		mux.HandleFunc("GET /api/v1/alert-rules/{id}", ch.GetAlertRule)
		mux.HandleFunc("PUT /api/v1/alert-rules/{id}", ch.UpdateAlertRule)
		mux.HandleFunc("DELETE /api/v1/alert-rules/{id}", ch.DeleteAlertRule)

		mux.HandleFunc("GET /api/v1/alerts", ch.ListAlerts)
		mux.HandleFunc("POST /api/v1/alerts/{id}/ack", ch.AcknowledgeAlert)
		mux.HandleFunc("POST /api/v1/alerts/{id}/resolve", ch.ResolveAlert)

		mux.HandleFunc("GET /api/v1/devices/{id}/state", ch.GetDeviceLatestState)
		mux.HandleFunc("GET /api/v1/devices/{id}/measurements", ch.GetDeviceMeasurements)
		mux.HandleFunc("GET /api/v1/devices/{id}/stats", ch.GetDeviceStats)
	}

	if cfg.IngestHandler != nil && cfg.DeviceService != nil {
		ih := cfg.IngestHandler
		authMiddleware := DeviceAuthMiddleware(cfg.DeviceService)
		sizeMiddleware := MaxBodySizeMiddleware(2 * 1024 * 1024)

		mux.Handle("POST /api/v1/telemetry", sizeMiddleware(authMiddleware(http.HandlerFunc(ih.IngestSingle))))
		mux.Handle("POST /api/v1/telemetry/batch", sizeMiddleware(authMiddleware(http.HandlerFunc(ih.IngestBatch))))
	}

	handler := RecoveryMiddleware(mux)
	handler = LoggingMiddleware(handler)

	return handler
}
