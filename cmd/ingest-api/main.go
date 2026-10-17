package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
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
	grpctransport "DevicePulse/internal/transport/grpc"
	"DevicePulse/internal/transport/grpc/pb"
	httptransport "DevicePulse/internal/transport/http"
	"google.golang.org/grpc"
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
	metricRepo := postgres.NewMetricRepo(db.Pool)
	telemetryRepo := postgres.NewTelemetryRepo(db.Pool)
	stateRepo := redis.NewStateRepo(redisClient)

	deviceService := service.NewDeviceService(deviceRepo, stateRepo)
	ingestService := service.NewIngestService(cfg, telemetryRepo, metricRepo, stateRepo, natsClient, metrics)

	ingestHandler := httptransport.NewIngestHandler(ingestService)
	router := httptransport.NewRouter(httptransport.RouterConfig{
		IngestHandler: ingestHandler,
		DeviceService: deviceService,
	})

	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("ingest-api http listening", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
			cancel()
		}
	}()

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		slog.Error("failed to listen on grpc port", "port", cfg.GRPCPort, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	telemetryServer := grpctransport.NewTelemetryServer(ingestService, deviceService)
	pb.RegisterTelemetryServiceServer(grpcServer, telemetryServer)

	go func() {
		slog.Info("ingest-api grpc listening", "port", cfg.GRPCPort)
		if err := grpcServer.Serve(grpcLis); err != nil {
			slog.Error("grpc server error", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down ingest-api")

	grpcServer.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
