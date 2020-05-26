package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

type channelGetter interface {
	GetByID(ctx context.Context, id string) (slack.Channel, bool, error)
	GetByName(ctx context.Context, name string) (slack.Channel, bool, error)
}

type channelPutter interface {
	Hash(ctx context.Context, id string) (string, bool, error)
	TTL(ctx context.Context, id string) (time.Duration, bool, error)
	Put(ctx context.Context, id, name, data, hash string) error
}

// ChannelFiller is channel cache filler.
type ChannelFiller struct {
	s     *slack.Client
	store channelPutter
	l     zerolog.Logger
}

// NewChannelFiller generates a new cache populator.
func NewChannelFiller(sc *slack.Client, rc *redis.Client, logger zerolog.Logger) (*ChannelFiller, error) {
	res := rc.Set(redisByIDPrefix+"populator_test_id_should_be_auto_removed", "foobar", time.Second)
	if err := res.Err(); err != nil {
		return nil, fmt.Errorf("failed to set test key: %w", err)
	}

	return &ChannelFiller{
		s:     sc,
		store: &store{r: rc},
		l:     logger,
	}, nil
}

var channelHasher = sha256.New()

func hashit(j []byte) string {
	defer channelHasher.Reset()
	if _, err := channelHasher.Write(j); err != nil {
		panic(err.Error())
	}
	return fmt.Sprintf("%x", channelHasher.Sum(nil))
}

// Fill loads the cache.
func (c *ChannelFiller) Fill(ctx context.Context) error {
	chans, err := c.s.GetChannelsContext(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get channel info: %w", err)
	}

	for _, ch := range chans {
		j, _ := json.Marshal(ch)
		h := hashit(j)

		rh, nf, err := c.store.Hash(ctx, ch.ID)
		if err != nil {
			return err
		}

		if nf {
			rh = ""
		}

		ttl, nf, err := c.store.TTL(ctx, ch.ID)
		if err != nil {
			return err
		}

		if nf {
			ttl = 0
		}

		expiry := time.Now().Add(ttl)
		threeDays := 3 * 24 * time.Hour

		// if the cache entry expires in more than 3 days
		// and the hash values are the same
		//
		// this way we refresh the cache to avoid the data expiring, but don't
		// needlessly update the data
		if time.Until(expiry) > threeDays && h == rh {
			continue
		}

		if err = c.store.Put(ctx, ch.ID, ch.Name, string(j), h); err != nil {
			return err
		}
	}

	c.l.Debug().
		Int("processed_count", len(chans)).
		Msg("processed channels")

	return nil
}

// Channel represents a Redis-backed channel cache.
type Channel struct {
	store channelGetter
}

// NewChannel creates a new channel cache.
func NewChannel(rc *redis.Client) *Channel {
	return &Channel{store: &store{r: rc}}
}

// Channel finds a channel by its ID in the cache. If the channel is not found,
// err will be nil and notFound true.
func (c *Channel) Channel(id string) (channel slack.Channel, notFound bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return c.store.GetByID(ctx, id)
}

// Lookup finds a channel by its name, without the #, in the cache. If the
// channel is not found, err will be nil and notFound true.
func (c *Channel) Lookup(name string) (slack.Channel, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return c.store.GetByName(ctx, name)
}
