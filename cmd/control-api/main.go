package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"DevicePulse/internal/config"
	"DevicePulse/internal/repository/postgres"
	"DevicePulse/internal/repository/redis"
	"DevicePulse/internal/service"
	httptransport "DevicePulse/internal/transport/http"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db, err := postgres.NewDB(ctx, cfg.PostgresURL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.RunMigrations(ctx); err != nil {
		slog.Error("failed to run database migrations", "error", err)
		os.Exit(1)
	}

	redisClient, err := redis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	deviceRepo := postgres.NewDeviceRepo(db.Pool)
	metricRepo := postgres.NewMetricRepo(db.Pool)
	alertRepo := postgres.NewAlertRepo(db.Pool)
	telemetryRepo := postgres.NewTelemetryRepo(db.Pool)
	stateRepo := redis.NewStateRepo(redisClient)

	deviceService := service.NewDeviceService(deviceRepo, stateRepo)
	metricService := service.NewMetricService(metricRepo)
	alertService := service.NewAlertService(alertRepo)

	controlHandler := httptransport.NewControlHandler(deviceService, metricService, alertService, telemetryRepo, stateRepo)

	router := httptransport.NewRouter(httptransport.RouterConfig{
		ControlHandler: controlHandler,
		DeviceService:  deviceService,
	})

	server := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("control-api listening", "port", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("control-api server error", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down control-api gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("control-api shutdown error", "error", err)
	}
}
