package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/slack-go/slack"
)

const (
	redisByIDPrefix   = "cache:channel:by_id:"
	redisByNamePrefix = "cache:channel:by_name:"
)

type store struct {
	r *redis.Client
}

func (s *store) Hash(ctx context.Context, id string) (string, bool, error) {
	key := fmt.Sprintf("%s%s:hash", redisByIDPrefix, id)

	res := s.r.Get(key)
	if err := res.Err(); err != nil {
		if err == redis.Nil {
			return "", true, nil
		}

		return "", false, fmt.Errorf("failed to get hash: %w", err)
	}

	hash, err := res.Result()
	if err != nil {
		return "", false, fmt.Errorf("failed to read result: %w", err)
	}

	return hash, false, nil
}

func (s *store) TTL(ctx context.Context, id string) (time.Duration, bool, error) {
	res := s.r.TTL(redisByIDPrefix + id)
	if err := res.Err(); err != nil {
		if err == redis.Nil {
			return 0, true, nil
		}

		return 0, false, fmt.Errorf("failed to get key: %w", err)
	}

	dur, err := res.Result()
	if err != nil {
		return 0, false, fmt.Errorf("failed to get duration result: %w", err)
	}

	return dur, false, nil
}

const channelCacheTTL = 14 * 24 * time.Hour // 14 days

func (s *store) Put(ctx context.Context, id, name, data, hash string) error {
	res := s.r.Set(redisByIDPrefix+id, data, channelCacheTTL)
	if err := res.Err(); err != nil {
		return fmt.Errorf("failed to set channel data: %w", err)
	}

	res = s.r.Set(redisByNamePrefix+name, id, channelCacheTTL)
	if err := res.Err(); err != nil {
		return fmt.Errorf("failed to set name to ID mapping: %w", err)
	}

	res = s.r.Set(redisByIDPrefix+id+":hash", hash, channelCacheTTL)
	if err := res.Err(); err != nil {
		return fmt.Errorf("failed to set channel data hash: %w", err)
	}

	return nil
}

func (s *store) GetByID(ctx context.Context, id string) (slack.Channel, bool, error) {
	res := s.r.Get(redisByIDPrefix + id)
	if err := res.Err(); err != nil {
		if err == redis.Nil {
			return slack.Channel{}, true, nil
		}

		return slack.Channel{}, false, fmt.Errorf("failed to get key: %w", err)
	}

	data, err := res.Bytes()
	if err != nil {
		return slack.Channel{}, false, fmt.Errorf("failed to read bytes from redis result: %w", err)
	}

	var sc slack.Channel
	if err = json.Unmarshal(data, &sc); err != nil {
		return slack.Channel{}, false, err
	}

	return sc, false, nil
}

func (s *store) GetByName(ctx context.Context, name string) (slack.Channel, bool, error) {
	res := s.r.Get(redisByNamePrefix + name)
	if err := res.Err(); err != nil {
		if err == redis.Nil {
			return slack.Channel{}, true, nil
		}

		return slack.Channel{}, false, fmt.Errorf("failed to get key: %w", err)
	}

	id, err := res.Result()
	if err != nil {
		return slack.Channel{}, false, fmt.Errorf("failed to read result: %w", err)
	}

	return s.GetByID(ctx, id)
}
