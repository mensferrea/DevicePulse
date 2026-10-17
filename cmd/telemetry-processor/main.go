package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"DevicePulse/internal/config"
	"DevicePulse/internal/queue"
	"DevicePulse/internal/repository/postgres"
	"DevicePulse/internal/repository/redis"
	"DevicePulse/internal/service"
	"DevicePulse/internal/telemetry"
	httptransport "DevicePulse/internal/transport/http"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	metrics := telemetry.NewMetrics()

	db, err := postgres.NewDB(ctx, cfg.PostgresURL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	redisClient, err := redis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	natsClient, err := queue.NewNatsClient(cfg.NatsURL, cfg.NatsStream)
	if err != nil {
		slog.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	deviceRepo := postgres.NewDeviceRepo(db.Pool)
	alertRepo := postgres.NewAlertRepo(db.Pool)
	telemetryRepo := postgres.NewTelemetryRepo(db.Pool)
	stateRepo := redis.NewStateRepo(redisClient)

	deviceService := service.NewDeviceService(deviceRepo, stateRepo)
	processorService := service.NewProcessorService(cfg, telemetryRepo, deviceRepo, alertRepo, stateRepo, natsClient, metrics)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				count, err := deviceService.CheckOfflineDevices(ctx, cfg.OfflineTimeout)
				if err != nil {
					slog.Error("error checking offline devices", "error", err)
				} else if count > 0 {
					slog.Info("devices marked offline", "count", count)
				}
			}
		}
	}()

	router := httptransport.NewRouter(httptransport.RouterConfig{})
	metricsServer := &http.Server{
		Addr:    ":" + cfg.MetricsPort,
		Handler: router,
	}

	go func() {
		slog.Info("telemetry-processor metrics listening", "port", cfg.MetricsPort)
		_ = metricsServer.ListenAndServe()
	}()

	slog.Info("starting telemetry processor worker pool")
	if err := processorService.Start(ctx); err != nil {
		slog.Error("processor worker pool terminated", "error", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = metricsServer.Shutdown(shutdownCtx)
}
