package redisclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"go_quant_system/pkg/logger"
)

type WeightLimit struct {
	Minute    int     `json:"minute"`
	LimitNum  float64 `json:"limit_num"`
	IsBlocked bool    `json:"is_blocked"`
}

type WeightRedisConfig struct {
	Addr                 string
	Password             string
	DBName               int
	ApiLimitKey          string
	CacheWindow          time.Duration
	WeightBlockThreshold float64
	WeightWarnThreshold  float64
	Timeout              time.Duration
}

type WeightRedisClient struct {
	cli    *redis.Client
	cfg    WeightRedisConfig
	logger logger.Logger
}

func NewWeightRedisClient(cfg WeightRedisConfig, log logger.Logger) *WeightRedisClient {
	return &WeightRedisClient{
		cli: redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DBName,
		}),
		cfg:    cfg,
		logger: log,
	}
}

func (r *WeightRedisClient) GetWeightLimit(ctx context.Context) (*WeightLimit, error) {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	val, err := r.cli.Get(ctx, r.cfg.ApiLimitKey).Result()
	if err != nil {
		if err == redis.Nil {
			return &WeightLimit{Minute: -1, LimitNum: 0, IsBlocked: false}, nil
		}
		r.logger.Error("redis get weight limit failed", logger.Err(err))
		return nil, err
	}

	var wl WeightLimit
	if err := json.Unmarshal([]byte(val), &wl); err != nil {
		r.logger.Error("redis unmarshal weight limit failed",
			logger.Err(err), logger.String("data", val))
		_ = r.cli.Del(ctx, r.cfg.ApiLimitKey).Err()
		return &WeightLimit{Minute: -1, LimitNum: 0, IsBlocked: false}, nil
	}

	return &wl, nil
}

func (r *WeightRedisClient) SetWeightLimit(ctx context.Context, wl *WeightLimit) error {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	data, err := json.Marshal(wl)
	if err != nil {
		r.logger.Error("redis marshal weight limit failed", logger.Err(err))
		return err
	}

	err = r.cli.Set(ctx, r.cfg.ApiLimitKey, string(data), 60*time.Second).Err()
	if err != nil {
		r.logger.Error("redis set weight limit failed", logger.Err(err))
	}
	return err
}

func (r *WeightRedisClient) Close() error {
	return r.cli.Close()
}
