// Package heartbeat provides a mechanism for heartbeating against Redis to
// ensure it's still healthy.
package heartbeat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/rs/zerolog"
)

const redisKeyFormat = "heartbeat:%s:%s"

type redisClient interface {
	Get(key string) *redis.StringCmd
	Set(key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

type Config struct {
	RedisClient redisClient
	Logger      zerolog.Logger
	AppName     string
	UID         string
	Warn        time.Duration
	Fail        time.Duration
	ShutdownFn  func(zerolog.Logger)
}

// Heart is the thing that beats.
type Heart struct {
	// d and Done are the same channel
	// it's captured twice as you can't call close() on a <-chan (Done)
	d chan struct{}

	// Done is closed when the Heart has stopped (due to Redis failure)
	Done <-chan struct{}

	ctx context.Context
	r   redisClient
	l   zerolog.Logger

	mu   *sync.Mutex
	last time.Time

	warn       time.Duration
	fail       time.Duration
	key        string
	shutdownFn func(zerolog.Logger)
}

// New is a function with an argument list that's roughly the size of Texas. The
// appName and UID are used as part of the key written to Redis. The warn and
// fail durations control when we log a warning, and when we start to exit
// (respectively). Finally, there's a shutdownFn if you want to clean up some
// things before the program exits. That function has 10 seconds to complete,
// otherwise the program is forcibly exited.
//
// If the error is not nil, the *Heart is beating.
func New(ctx context.Context, cfg Config) (*Heart, error) {
	if len(cfg.UID) == 0 {
		return nil, fmt.Errorf("must provide cfg.UID to New()")
	}

	if cfg.RedisClient == nil {
		return nil, fmt.Errorf("must provide a cfg.RedisClient")
	}

	d := make(chan struct{})

	h := &Heart{
		d:          d,
		Done:       d,
		ctx:        ctx,
		r:          cfg.RedisClient,
		l:          cfg.Logger,
		mu:         &sync.Mutex{},
		warn:       cfg.Warn,
		fail:       cfg.Fail,
		key:        fmt.Sprintf(redisKeyFormat, cfg.AppName, cfg.UID),
		shutdownFn: cfg.ShutdownFn,
	}

	if err := h.beat(); err != nil {
		return nil, fmt.Errorf("initial beat error: %w", err)
	}

	go h.pulse()
	go h.monitor()

	return h, nil
}

func (h *Heart) monitor() {
	t := time.NewTicker(time.Second)

	var warn bool

	for {
		select {
		case <-h.ctx.Done():
			t.Stop()
			return
		case <-t.C:
			// escape out to for loop
		}

		h.mu.Lock()
		last := h.last
		h.mu.Unlock()

		dur := time.Since(last)

		switch {
		case dur < h.warn:
			warn = false

		case dur >= h.warn && !warn:
			h.l.Warn().
				Str("since_last_heartbeat", dur.String()).
				Msg("redis heart beat is going stale")

			warn = true

		case dur >= h.fail && warn:
			h.l.WithLevel(zerolog.FatalLevel).
				Str("since_last_heartbeat", dur.String()).
				Msg("redis connection might be gone; trying to cleanly shut down")

			h.bailout()
		}
	}
}

func (h *Heart) bailout() {
	// implicitly closes h.Done as they are same channel
	defer close(h.d)

	msg := "cleanly exiting"

	if h.shutdownFn != nil {
		d := make(chan struct{})

		t := time.NewTimer(10 * time.Second)

		go func() {
			h.shutdownFn(h.l)
			close(d)
		}()

		select {
		case <-d:
			t.Stop()
		case <-t.C:
			msg = "clean shutdown timed out, exiting now!"
		}
	}

	h.l.Fatal().Msg(msg)
}

func (h *Heart) pulse() {
	h.l.Info().Msg("starting 1 second pulses")

	t := time.NewTimer(1 * time.Second)

	for {
		select {
		case <-h.ctx.Done():
			t.Stop()
			return
		case <-t.C:
			// escape out to for loop
		}

		if err := h.beat(); err != nil {
			h.l.Error().Err(err).
				Msg("heartbeat failed")
		}

		t.Reset(time.Second)
	}
}

func (h *Heart) beat() error {
	tn := time.Now().UnixNano() / int64(time.Millisecond)

	status := h.r.Set(h.key, tn, h.fail+time.Minute)
	if err := status.Err(); err != nil {
		return fmt.Errorf("failed to beat: %w", err)
	}

	res := h.r.Get(h.key)
	if err := res.Err(); err != nil {
		return fmt.Errorf("failed to read beat: %w", err)
	}

	ts, err := res.Int64()
	if err != nil {
		return fmt.Errorf("failed to read timestamp from redis: %w", err)
	}

	if ts != tn {
		return fmt.Errorf("ts = %d, want %d", ts, tn)
	}

	t := time.Unix(unix(ts))

	h.mu.Lock()
	defer h.mu.Unlock()

	h.last = t

	return nil
}

func unix(i int64) (int64, int64) {
	// convert milliseconds to whole seconds
	// convert millisecond remainder from above conversion to nanoseconds
	return i / 1000, (i % 1000) * int64(time.Millisecond)
}
