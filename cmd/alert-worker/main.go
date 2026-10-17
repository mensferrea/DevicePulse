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

	natsClient, err := queue.NewNatsClient(cfg.NatsURL, cfg.NatsStream)
	if err != nil {
		slog.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	webhookRepo := postgres.NewWebhookRepo(db.Pool)
	webhookService := service.NewWebhookService(cfg, webhookRepo, natsClient, metrics)

	router := httptransport.NewRouter(httptransport.RouterConfig{})
	workerHTTPPort := "9092"
	server := &http.Server{
		Addr:    ":" + workerHTTPPort,
		Handler: router,
	}

	go func() {
		slog.Info("alert-worker metrics listening", "port", workerHTTPPort)
		_ = server.ListenAndServe()
	}()

	slog.Info("starting alert-worker webhook processor")
	if err := webhookService.Start(ctx); err != nil {
		slog.Error("alert worker terminated", "error", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
}
