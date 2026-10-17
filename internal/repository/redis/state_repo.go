package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"DevicePulse/internal/domain"
	"github.com/redis/go-redis/v9"
)

type StateRepo struct {
	client *Client
}

func NewStateRepo(client *Client) *StateRepo {
	return &StateRepo{client: client}
}

func (r *StateRepo) SetLatestPoint(ctx context.Context, deviceID, metric string, point domain.LatestPoint) error {
	data, err := json.Marshal(point)
	if err != nil {
		return fmt.Errorf("marshal point: %w", err)
	}

	key := fmt.Sprintf("device:%s:metrics", deviceID)
	err = r.client.RDB.HSet(ctx, key, metric, data).Err()
	if err != nil {
		return fmt.Errorf("hset metric point: %w", err)
	}
	return nil
}

func (r *StateRepo) GetLatestState(ctx context.Context, deviceID string) (*domain.DeviceLatestState, error) {
	metricsKey := fmt.Sprintf("device:%s:metrics", deviceID)
	statusKey := fmt.Sprintf("device:%s:status", deviceID)
	lastSeenKey := fmt.Sprintf("device:%s:last_seen", deviceID)

	pipe := r.client.RDB.Pipeline()
	metricsCmd := pipe.HGetAll(ctx, metricsKey)
	statusCmd := pipe.Get(ctx, statusKey)
	lastSeenCmd := pipe.Get(ctx, lastSeenKey)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("get latest state pipe exec: %w", err)
	}

	state := &domain.DeviceLatestState{
		DeviceID: deviceID,
		Status:   domain.DeviceStatusUnknown,
		Metrics:  make(map[string]domain.LatestPoint),
	}

	if st, err := statusCmd.Result(); err == nil && st != "" {
		state.Status = domain.DeviceStatus(st)
	}

	if lsStr, err := lastSeenCmd.Result(); err == nil && lsStr != "" {
		if lsUnix, err := strconv.ParseInt(lsStr, 10, 64); err == nil {
			t := time.Unix(0, lsUnix).UTC()
			state.LastSeenAt = &t
		}
	}

	metricsMap, err := metricsCmd.Result()
	if err == nil {
		for k, v := range metricsMap {
			var pt domain.LatestPoint
			if err := json.Unmarshal([]byte(v), &pt); err == nil {
				state.Metrics[k] = pt
			}
		}
	}

	return state, nil
}

func (r *StateRepo) SetDeviceStatus(ctx context.Context, deviceID string, status domain.DeviceStatus, lastSeen time.Time) error {
	statusKey := fmt.Sprintf("device:%s:status", deviceID)
	lastSeenKey := fmt.Sprintf("device:%s:last_seen", deviceID)

	pipe := r.client.RDB.Pipeline()
	pipe.Set(ctx, statusKey, string(status), 0)
	pipe.Set(ctx, lastSeenKey, lastSeen.UnixNano(), 0)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("set device status pipe: %w", err)
	}
	return nil
}

func (r *StateRepo) GetDeviceStatus(ctx context.Context, deviceID string) (domain.DeviceStatus, *time.Time, error) {
	statusKey := fmt.Sprintf("device:%s:status", deviceID)
	lastSeenKey := fmt.Sprintf("device:%s:last_seen", deviceID)

	pipe := r.client.RDB.Pipeline()
	statusCmd := pipe.Get(ctx, statusKey)
	lastSeenCmd := pipe.Get(ctx, lastSeenKey)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return domain.DeviceStatusUnknown, nil, fmt.Errorf("get status pipe: %w", err)
	}

	status := domain.DeviceStatusUnknown
	if st, err := statusCmd.Result(); err == nil && st != "" {
		status = domain.DeviceStatus(st)
	}

	var lastSeen *time.Time
	if lsStr, err := lastSeenCmd.Result(); err == nil && lsStr != "" {
		if lsUnix, err := strconv.ParseInt(lsStr, 10, 64); err == nil {
			t := time.Unix(0, lsUnix).UTC()
			lastSeen = &t
		}
	}

	return status, lastSeen, nil
}

func (r *StateRepo) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	currentWindow := time.Now().Unix() / int64(window.Seconds())
	redisKey := fmt.Sprintf("ratelimit:%s:%d", key, currentWindow)

	pipe := r.client.RDB.Pipeline()
	incrCmd := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, window*2)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, fmt.Errorf("rate limit pipeline: %w", err)
	}

	count := incrCmd.Val()
	return count <= int64(limit), nil
}

func (r *StateRepo) IsMeasurementSeen(ctx context.Context, deviceID, metric string, recordedAt time.Time, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("dedup:%s:%s:%d", deviceID, metric, recordedAt.UnixNano())
	set, err := r.client.RDB.SetNX(ctx, key, 1, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("check dedup: %w", err)
	}
	return !set, nil
}
