package http

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"DevicePulse/internal/service"
)

type contextKey string

const DeviceIDContextKey contextKey = "deviceID"

func DeviceIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(DeviceIDContextKey).(string); ok {
		return val
	}
	return ""
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Debug("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"duration", time.Since(start),
		)
	})
}

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered in http handler", "panic", rec)
				WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func MaxBodySizeMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func DeviceAuthMiddleware(deviceService *service.DeviceService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-Device-Key")
			if apiKey == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					apiKey = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if apiKey == "" {
				WriteError(w, http.StatusUnauthorized, "missing device api key")
				return
			}

			deviceID, err := deviceService.Authenticate(r.Context(), apiKey)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "invalid or revoked device api key")
				return
			}

			ctx := context.WithValue(r.Context(), DeviceIDContextKey, deviceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
