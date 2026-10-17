package grpc

import (
	"context"
	"io"
	"strings"
	"time"

	"DevicePulse/internal/domain"
	"DevicePulse/internal/service"
	"DevicePulse/internal/transport/grpc/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type TelemetryServer struct {
	pb.UnimplementedTelemetryServiceServer
	ingestService *service.IngestService
	deviceService *service.DeviceService
}

func NewTelemetryServer(is *service.IngestService, ds *service.DeviceService) *TelemetryServer {
	return &TelemetryServer{
		ingestService: is,
		deviceService: ds,
	}
}

func (s *TelemetryServer) authenticate(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "metadata missing")
	}

	var apiKey string
	if vals := md.Get("x-device-key"); len(vals) > 0 {
		apiKey = vals[0]
	} else if vals := md.Get("authorization"); len(vals) > 0 {
		if strings.HasPrefix(vals[0], "Bearer ") {
			apiKey = strings.TrimPrefix(vals[0], "Bearer ")
		} else {
			apiKey = vals[0]
		}
	}

	if apiKey == "" {
		return "", status.Error(codes.Unauthenticated, "missing api key in metadata")
	}

	deviceID, err := s.deviceService.Authenticate(ctx, apiKey)
	if err != nil {
		return "", status.Error(codes.Unauthenticated, "invalid or revoked api key")
	}

	return deviceID, nil
}

func (s *TelemetryServer) SendBatch(ctx context.Context, req *pb.TelemetryBatchRequest) (*pb.TelemetryBatchResponse, error) {
	deviceID, err := s.authenticate(ctx)
	if err != nil {
		return nil, err
	}

	if req.DeviceId != "" && req.DeviceId != deviceID {
		return nil, status.Error(codes.PermissionDenied, "device id mismatch")
	}

	measurements := make([]domain.Measurement, len(req.Measurements))
	for i, item := range req.Measurements {
		var recTime time.Time
		if item.Timestamp != nil {
			recTime = item.Timestamp.AsTime()
		} else {
			recTime = time.Now().UTC()
		}

		measurements[i] = domain.Measurement{
			DeviceID:   deviceID,
			Metric:     item.Metric,
			Value:      item.Value,
			Unit:       item.Unit,
			RecordedAt: recTime,
		}
	}

	err = s.ingestService.Ingest(ctx, deviceID, req.BatchId, measurements, "grpc")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ingest error: %v", err)
	}

	return &pb.TelemetryBatchResponse{
		Success:        true,
		AcceptedCount:  int32(len(measurements)),
		Message:        "batch processed successfully",
	}, nil
}

func (s *TelemetryServer) SendStream(stream pb.TelemetryService_SendStreamServer) error {
	ctx := stream.Context()
	deviceID, err := s.authenticate(ctx)
	if err != nil {
		return err
	}

	var totalAccepted int64

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.TelemetryStreamResponse{
				Success:       true,
				TotalAccepted: totalAccepted,
				Message:       "stream completed successfully",
			})
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "stream read error: %v", err)
		}

		measurements := make([]domain.Measurement, len(req.Measurements))
		for i, item := range req.Measurements {
			var recTime time.Time
			if item.Timestamp != nil {
				recTime = item.Timestamp.AsTime()
			} else {
				recTime = time.Now().UTC()
			}

			measurements[i] = domain.Measurement{
				DeviceID:   deviceID,
				Metric:     item.Metric,
				Value:      item.Value,
				Unit:       item.Unit,
				RecordedAt: recTime,
			}
		}

		if err := s.ingestService.Ingest(ctx, deviceID, req.BatchId, measurements, "grpc-stream"); err != nil {
			return status.Errorf(codes.Internal, "ingest stream item error: %v", err)
		}

		totalAccepted += int64(len(measurements))
	}
}
