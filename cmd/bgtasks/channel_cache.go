package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/gobridge/gopherbot/cache"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

func setUpChannelCacheFiller(ctx context.Context, logger zerolog.Logger, sc *slack.Client, rc *redis.Client) (chan struct{}, error) {
	logger = logger.With().Str("context", "channel_cache_filler").Logger()

	filler, err := cache.NewChannelFiller(sc, rc, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build cache filler: %w", err)
	}

	t := time.NewTimer(0)
	w := make(chan struct{})

	go func() {
		logger.Info().Msg("starting channel cache filler")

		for {
			select {
			case <-t.C:
				gctx, cancel := context.WithTimeout(ctx, 10*time.Second)

				err := filler.Fill(gctx)

				cancel()

				t.Reset(10 * time.Minute)

				if err != nil {
					logger.Error().
						Err(err).
						Msg("trying cache fille again in 10 minutes")

					continue
				}

				logger.Trace().
					Msg("cache fill again in 10 minutes")

			case <-ctx.Done():
				defer close(w)

				logger.Info().
					Err(ctx.Err()).
					Msg("context canceled: shutting down poller")

				return
			}
		}
	}()

	return w, nil
}
