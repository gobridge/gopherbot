package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
	"github.com/gobridge/gopherbot/internal/poller/gotime"
)

const (
	gotimeGopherdevChannelID = "C013XC5SU21"

	// XXX(theckman): UNDO THIS
	// golangclsChannelID = "C0F1752BB"
	gotimeChannelID = gotimeGopherdevChannelID
)

const goTimeMsg = ":tada: GoTimeFM is now live :tada:"

func goTimeNotifyFactory(c *slack.Client, channelID string) gotime.NotifyFunc {
	return func(ctx context.Context) error {
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

	gp, err := gotime.New(gs, newHTTPClient(), logger, 30*time.Second, goTimeNotifyFactory(sc, cid))
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
