package app

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

func Run() {
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

	natsClient, err := queue.NewNatsClient(cfg.NatsURL, cfg.NatsStream)
	if err != nil {
		slog.Error("failed to connect to nats", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	deviceRepo := postgres.NewDeviceRepo(db.Pool)
	metricRepo := postgres.NewMetricRepo(db.Pool)
	telemetryRepo := postgres.NewTelemetryRepo(db.Pool)
	alertRepo := postgres.NewAlertRepo(db.Pool)
	webhookRepo := postgres.NewWebhookRepo(db.Pool)
	stateRepo := redis.NewStateRepo(redisClient)

	deviceService := service.NewDeviceService(deviceRepo, stateRepo)
	metricService := service.NewMetricService(metricRepo)
	alertService := service.NewAlertService(alertRepo)
	ingestService := service.NewIngestService(cfg, telemetryRepo, metricRepo, stateRepo, natsClient, metrics)
	processorService := service.NewProcessorService(cfg, telemetryRepo, deviceRepo, alertRepo, stateRepo, natsClient, metrics)
	webhookService := service.NewWebhookService(cfg, webhookRepo, natsClient, metrics)

	controlHandler := httptransport.NewControlHandler(deviceService, metricService, alertService, telemetryRepo, stateRepo)
	ingestHandler := httptransport.NewIngestHandler(ingestService)

	router := httptransport.NewRouter(httptransport.RouterConfig{
		ControlHandler: controlHandler,
		IngestHandler:  ingestHandler,
		DeviceService:  deviceService,
	})

	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("all-in-one http server listening", "port", cfg.HTTPPort)
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
		slog.Info("all-in-one grpc server listening", "port", cfg.GRPCPort)
		if err := grpcServer.Serve(grpcLis); err != nil {
			slog.Error("grpc server error", "error", err)
			cancel()
		}
	}()

	go func() {
		slog.Info("starting telemetry processor")
		if err := processorService.Start(ctx); err != nil {
			slog.Error("processor worker error", "error", err)
		}
	}()

	go func() {
		slog.Info("starting webhook worker")
		if err := webhookService.Start(ctx); err != nil {
			slog.Error("webhook worker error", "error", err)
		}
	}()

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

	slog.Info("DevicePulse all-in-one running")
	<-ctx.Done()
	slog.Info("shutting down all-in-one services")

	grpcServer.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
