package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment      string
	LogLevel         string
	HTTPPort         string
	GRPCPort         string
	MetricsPort      string
	PostgresURL      string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	NatsURL          string
	NatsStream       string
	NatsSubjectRaw   string
	NatsSubjectAlert string
	OfflineTimeout   time.Duration
	RateLimitMaxRPS  int
	RateLimitBurst   int
	MaxBatchSize     int
	WorkerPoolSize   int
}

func Load() *Config {
	return &Config{
		Environment:      getEnv("APP_ENV", "development"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		HTTPPort:         getEnv("HTTP_PORT", "8080"),
		GRPCPort:         getEnv("GRPC_PORT", "50051"),
		MetricsPort:      getEnv("METRICS_PORT", "9090"),
		PostgresURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/devicepulse?sslmode=disable"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getEnvInt("REDIS_DB", 0),
		NatsURL:          getEnv("NATS_URL", "nats://localhost:4222"),
		NatsStream:       getEnv("NATS_STREAM", "TELEMETRY"),
		NatsSubjectRaw:   getEnv("NATS_SUBJECT_RAW", "telemetry.raw"),
		NatsSubjectAlert: getEnv("NATS_SUBJECT_ALERT", "telemetry.alerts"),
		OfflineTimeout:   getEnvDuration("OFFLINE_TIMEOUT", 60*time.Second),
		RateLimitMaxRPS:  getEnvInt("RATE_LIMIT_RPS", 100),
		RateLimitBurst:   getEnvInt("RATE_LIMIT_BURST", 200),
		MaxBatchSize:     getEnvInt("MAX_BATCH_SIZE", 1000),
		WorkerPoolSize:   getEnvInt("WORKER_POOL_SIZE", 10),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}
