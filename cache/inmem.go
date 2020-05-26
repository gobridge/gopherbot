package cache

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// InMemChannel represents a channel info cache.
type InMemChannel struct {
	sc *slack.Client
	l  zerolog.Logger

	mu        *sync.RWMutex
	channels  map[string]slack.Channel
	chanNames map[string]string
}

// NewInMemChannel returns a new channel cache.
func NewInMemChannel(sc *slack.Client, l zerolog.Logger) (*InMemChannel, error) {
	c := &InMemChannel{
		sc: sc,
		l:  l,
		mu: &sync.RWMutex{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	err := c.update(ctx)

	cancel()
	if err != nil {
		return nil, fmt.Errorf("failed to init channel cache: %w", err)
	}

	go c.poller()

	return c, nil
}

func (s *InMemChannel) update(ctx context.Context) error {
	chans, err := s.sc.GetChannelsContext(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get channels: %w", err)
	}

	cs := make(map[string]slack.Channel, len(chans))
	cn := make(map[string]string, len(chans))

	for _, c := range chans {
		cs[c.ID] = c
		cn[c.Name] = c.ID
	}

	s.l.Debug().
		Int("channel_count", len(chans)).
		Msg("updating refreshed channel list")

	s.mu.Lock()
	defer s.mu.Unlock()

	s.channels = cs
	s.chanNames = cn

	return nil
}

func (s *InMemChannel) poller() {
	const minDelay = 5 * time.Minute
	splay := minDelay + (time.Duration(rand.Intn(300)) * time.Second)

	timer := time.NewTimer(splay)
	s.l.Info().
		Str("delay", splay.String()).
		Msg("set timer")

	for {
		<-timer.C
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

		err := s.update(ctx)

		cancel()

		if err != nil {
			s.l.Error().
				Err(err).
				Msg("failed to refresh channels")
		}

		splay = minDelay + (time.Duration(rand.Intn(300)) * time.Second)
		timer.Reset(splay)
		s.l.Info().
			Str("delay", splay.String()).
			Msg("reset timer")

	}
}

// Lookup finds a channel by name.
func (s *InMemChannel) Lookup(name string) (slack.Channel, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.chanNames[name]
	if !ok {
		return slack.Channel{}, true, nil
	}

	c, ok := s.channels[id]
	if !ok {
		return slack.Channel{}, true, nil
	}

	return c, false, nil
}
