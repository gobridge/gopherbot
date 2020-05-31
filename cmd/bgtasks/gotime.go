package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/gobridge/gopherbot/internal/poller/gotime"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

const (
	gotimeGopherdevChannelID = "C013XC5SU21"
	gotimeChannelID          = "C0F1752BB"
)

const goTimeMsg = ":tada: GoTimeFM is now live :tada:"

func goTimeNotifyFactory(logger zerolog.Logger, c *slack.Client, channelID string, shadowMode bool) gotime.NotifyFunc {
	return func(ctx context.Context) error {
		if shadowMode {
			logger.Info().
				Bool("shadow_mode", true).
				Msg("would announce it's GoTime!")

			return nil
		}

		opts := []slack.MsgOption{
			slack.MsgOptionText(goTimeMsg, false),
		}

		_, _, _, err := c.SendMessageContext(ctx, channelID, opts...)

		return err
	}
}

func setUpGoTime(ctx context.Context, shadowMode bool, logger zerolog.Logger, sc *slack.Client, rc *redis.Client) (chan struct{}, error) {
	gs, err := gotime.NewStore(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to build gotime store: %w", err)
	}

	logger = logger.With().Str("context", "gotime_poller").Logger()

	cid := gotimeChannelID
	if shadowMode {
		cid = gotimeGopherdevChannelID
	}

	ln := logger.With().Str("context", "gotime_notifier").Logger()
	gp, err := gotime.New(gs, newHTTPClient(), logger, 30*time.Second, goTimeNotifyFactory(ln, sc, cid, shadowMode))
	if err != nil {
		return nil, fmt.Errorf("failed to create new gotime poller: %w", err)
	}

	t := time.NewTimer(0)
	w := make(chan struct{})

	go func() {
		logger.Info().Msg("starting GoTime poller")

		for {
			select {
			case <-t.C:
				gctx, cancel := context.WithTimeout(ctx, 10*time.Second)

				err := gp.Poll(gctx)

				cancel()

				t.Reset(time.Minute)

				if err != nil {
					logger.Error().
						Err(err).
						Msg("trying GoTime poll again in 1 minute")

					continue
				}

				logger.Trace().
					Msg("polling GoTime in 1 minute")

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
