package gotimestatus

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

const (
	redisKey     = "poller:gotime:last_status"
	redisTestKey = "poller:gotime:test_key"
)

// DefaultStore is a default implementation of the Store interface.
type DefaultStore struct {
	r *redis.Client
}

var _ Store = (*DefaultStore)(nil)

// NewStore returns a new DefaultStore.
func NewStore(rc *redis.Client) (*DefaultStore, error) {
	res := rc.Set(redisTestKey, "foobar", 1*time.Second)

	if err := res.Err(); err != nil {
		return nil, fmt.Errorf("failed to write to redis: %w", err)
	}

	return &DefaultStore{r: rc}, nil
}

// Get satisfies Store.
func (s *DefaultStore) Get(ctx context.Context) (string, bool, error) {
	select {
	case <-ctx.Done():
		return "", false, ctx.Err()
	default:
		// noop
	}

	res := s.r.Get(redisKey)
	if err := res.Err(); err != nil {
		if err == redis.Nil {
			return "", true, nil
		}

		return "", false, fmt.Errorf("failed to GET redis key: %w", err)
	}

	select {
	case <-ctx.Done():
		return "", false, ctx.Err()
	default:
		// noop
	}

	v, err := res.Result()
	if err != nil {
		return "", false, fmt.Errorf("key found, but was not int64: %w", err)
	}

	return v, false, nil
}

// Put satisfies Store.
func (s *DefaultStore) Put(ctx context.Context, id string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// noop
	}

	// set for 31 days
	res := s.r.Set(redisKey, id, 31*24*time.Hour)

	if err := res.Err(); err != nil {
		return fmt.Errorf("failed to set last ID %s: %w", id, err)
	}

	return nil
}
