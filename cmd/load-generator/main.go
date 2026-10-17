package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"DevicePulse/internal/transport/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Config struct {
	Transport    string
	Target       string
	Workers      int
	Duration     time.Duration
	BatchSize    int
	RatePerWorker int
	APIKey       string
	DeviceID     string
	AutoRegister bool
	ControlURL   string
}

type MetricsCollector struct {
	mu           sync.Mutex
	latencies    []time.Duration
	totalReqs    int64
	successReqs  int64
	failedReqs   int64
	measurements int64
}

func (m *MetricsCollector) Record(dur time.Duration, success bool, count int) {
	if success {
		atomic.AddInt64(&m.successReqs, 1)
	} else {
		atomic.AddInt64(&m.failedReqs, 1)
	}
	atomic.AddInt64(&m.totalReqs, 1)
	atomic.AddInt64(&m.measurements, int64(count))

	m.mu.Lock()
	m.latencies = append(m.latencies, dur)
	m.mu.Unlock()
}

func (m *MetricsCollector) PrintReport(totalTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sort.Slice(m.latencies, func(i, j int) bool {
		return m.latencies[i] < m.latencies[j]
	})

	n := len(m.latencies)
	var avg, p50, p95, p99, min, max time.Duration

	if n > 0 {
		min = m.latencies[0]
		max = m.latencies[n-1]
		var sum time.Duration
		for _, d := range m.latencies {
			sum += d
		}
		avg = sum / time.Duration(n)
		p50 = m.latencies[n*50/100]
		p95 = m.latencies[n*95/100]
		p99 = m.latencies[n*99/100]
	}

	reqSec := float64(m.totalReqs) / totalTime.Seconds()
	measSec := float64(m.measurements) / totalTime.Seconds()

	fmt.Println("\n================ LOAD TEST RESULTS ================")
	fmt.Printf("Elapsed Time:         %v\n", totalTime)
	fmt.Printf("Total Requests:       %d\n", m.totalReqs)
	fmt.Printf("Successful Requests:  %d (%.2f%%)\n", m.successReqs, float64(m.successReqs)*100/float64(maxInt(1, m.totalReqs)))
	fmt.Printf("Failed Requests:      %d\n", m.failedReqs)
	fmt.Printf("Total Measurements:   %d\n", m.measurements)
	fmt.Printf("Throughput (Req/s):   %.2f req/s\n", reqSec)
	fmt.Printf("Throughput (Data/s):  %.2f measurements/s\n", measSec)
	fmt.Println("----------------- LATENCY DISTRIBUTION -----------------")
	fmt.Printf("Min:                  %v\n", min)
	fmt.Printf("Avg:                  %v\n", avg)
	fmt.Printf("p50 (Median):         %v\n", p50)
	fmt.Printf("p95:                  %v\n", p95)
	fmt.Printf("p99:                  %v\n", p99)
	fmt.Printf("Max:                  %v\n", max)
	fmt.Println("====================================================")
}

func maxInt(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func main() {
	var cfg Config
	flag.StringVar(&cfg.Transport, "transport", "rest", "Transport protocol: rest or grpc")
	flag.StringVar(&cfg.Target, "target", "http://localhost:8080", "Target server base URL or host:port")
	flag.IntVar(&cfg.Workers, "workers", 10, "Number of concurrent worker goroutines")
	flag.DurationVar(&cfg.Duration, "duration", 10*time.Second, "Duration of the test")
	flag.IntVar(&cfg.BatchSize, "batch-size", 10, "Number of measurements per batch")
	flag.IntVar(&cfg.RatePerWorker, "rate", 0, "Max requests per second per worker (0 for unlimited)")
	flag.StringVar(&cfg.APIKey, "api-key", "", "Device API key")
	flag.StringVar(&cfg.DeviceID, "device-id", "", "Device ID")
	flag.BoolVar(&cfg.AutoRegister, "auto-register", true, "Auto-register test devices before testing")
	flag.StringVar(&cfg.ControlURL, "control-url", "http://localhost:8080", "Control API URL for auto registration")
	flag.Parse()

	deviceKeys := make([]string, cfg.Workers)
	deviceIDs := make([]string, cfg.Workers)

	if cfg.AutoRegister {
		fmt.Printf("Auto-registering %d test devices via %s...\n", cfg.Workers, cfg.ControlURL)
		client := &http.Client{Timeout: 5 * time.Second}
		for i := 0; i < cfg.Workers; i++ {
			body, _ := json.Marshal(map[string]string{
				"name":  fmt.Sprintf("load-device-%d", i+1),
				"group": "load-test-group",
			})
			resp, err := client.Post(cfg.ControlURL+"/api/v1/devices", "application/json", bytes.NewBuffer(body))
			if err != nil {
				fmt.Printf("Error registering device: %v\n", err)
				return
			}
			var res struct {
				Device struct {
					ID string `json:"id"`
				} `json:"device"`
				APIKey string `json:"api_key"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&res)
			resp.Body.Close()

			deviceIDs[i] = res.Device.ID
			deviceKeys[i] = res.APIKey
		}
		fmt.Println("Registration complete. Starting load generation...")
	} else {
		for i := 0; i < cfg.Workers; i++ {
			deviceKeys[i] = cfg.APIKey
			deviceIDs[i] = cfg.DeviceID
		}
	}

	collector := &MetricsCollector{
		latencies: make([]time.Duration, 0, 100000),
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		workerID := i
		devID := deviceIDs[workerID]
		apiKey := deviceKeys[workerID]

		go func() {
			defer wg.Done()

			if cfg.Transport == "grpc" {
				runGRPCWorker(ctx, cfg, devID, apiKey, collector)
			} else {
				runRESTWorker(ctx, cfg, devID, apiKey, collector)
			}
		}()
	}

	wg.Wait()
	totalTime := time.Since(startTime)
	collector.PrintReport(totalTime)
}

func runRESTWorker(ctx context.Context, cfg Config, devID, apiKey string, collector *MetricsCollector) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	metricsList := []string{"temperature", "humidity", "pressure", "voltage"}
	url := cfg.Target + "/api/v1/telemetry/batch"

	var ticker *time.Ticker
	if cfg.RatePerWorker > 0 {
		ticker = time.NewTicker(time.Second / time.Duration(cfg.RatePerWorker))
		defer ticker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if ticker != nil {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}

			measurements := make([]map[string]any, cfg.BatchSize)
			now := time.Now().UTC()
			for j := 0; j < cfg.BatchSize; j++ {
				metric := metricsList[rand.Intn(len(metricsList))]
				val := 20.0 + rand.Float64()*15.0
				measurements[j] = map[string]any{
					"metric":      metric,
					"value":       val,
					"unit":        "standard",
					"recorded_at": now.Add(time.Duration(j) * time.Millisecond),
				}
			}

			body, _ := json.Marshal(map[string]any{
				"batch_id":     fmt.Sprintf("batch-%d", time.Now().UnixNano()),
				"measurements": measurements,
			})

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
			if err != nil {
				collector.Record(0, false, cfg.BatchSize)
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Device-Key", apiKey)

			start := time.Now()
			resp, err := client.Do(req)
			dur := time.Since(start)

			if err != nil {
				collector.Record(dur, false, cfg.BatchSize)
				continue
			}

			success := resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK
			_ = resp.Body.Close()
			collector.Record(dur, success, cfg.BatchSize)
		}
	}
}

func runGRPCWorker(ctx context.Context, cfg Config, devID, apiKey string, collector *MetricsCollector) {
	conn, err := grpc.NewClient(cfg.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("grpc connect error: %v\n", err)
		return
	}
	defer conn.Close()

	client := pb.NewTelemetryServiceClient(conn)
	metricsList := []string{"temperature", "humidity", "pressure", "voltage"}

	var ticker *time.Ticker
	if cfg.RatePerWorker > 0 {
		ticker = time.NewTicker(time.Second / time.Duration(cfg.RatePerWorker))
		defer ticker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if ticker != nil {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}

			items := make([]*pb.MeasurementItem, cfg.BatchSize)
			now := time.Now().UTC()
			for j := 0; j < cfg.BatchSize; j++ {
				metric := metricsList[rand.Intn(len(metricsList))]
				val := 20.0 + rand.Float64()*15.0
				items[j] = &pb.MeasurementItem{
					Metric:    metric,
					Value:     val,
					Unit:      "standard",
					Timestamp: timestamppb.New(now.Add(time.Duration(j) * time.Millisecond)),
				}
			}

			req := &pb.TelemetryBatchRequest{
				DeviceId:     devID,
				BatchId:      fmt.Sprintf("grpc-batch-%d", time.Now().UnixNano()),
				SentAt:       timestamppb.New(now),
				Measurements: items,
			}

			md := metadata.Pairs("x-device-key", apiKey)
			callCtx := metadata.NewOutgoingContext(ctx, md)

			start := time.Now()
			resp, err := client.SendBatch(callCtx, req)
			dur := time.Since(start)

			success := err == nil && resp != nil && resp.Success
			collector.Record(dur, success, cfg.BatchSize)
		}
	}
}
