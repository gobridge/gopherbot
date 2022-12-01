package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/gobridge/gopherbot/internal/poller/gotimestatus"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

func goTimeStatusNotifyFactory(logger zerolog.Logger, c *slack.Client, channelID string, shadowMode bool) gotimestatus.NotifyFunc {
	return func(ctx context.Context, statusURL string) error {
		if shadowMode {
			logger.Info().
				Bool("shadow_mode", true).
				Msgf("would send GoTime Social status %s", statusURL)

			return nil
		}

		// urls must be enclosed in `<>`. See: https://api.slack.com/reference/messaging/link-unfurling
		text := fmt.Sprintf("<%s>", statusURL)
		opts := []slack.MsgOption{
			slack.MsgOptionUsername("Changelog"),
			slack.MsgOptionIconURL("https://cdn.changelog.social/accounts/avatars/109/365/688/871/983/824/original/5d1bcf4960706353.png"),
			slack.MsgOptionText(text, false), // don't escape, otherwise the link will break and won't unfurl
			slack.MsgOptionEnableLinkUnfurl(),
		}

		_, _, _, err := c.SendMessageContext(ctx, channelID, opts...)

		return err
	}
}

func setUpGoTimeStatus(ctx context.Context, shadowMode bool, logger zerolog.Logger, sc *slack.Client, rc *redis.Client) (chan struct{}, error) {
	gs, err := gotimestatus.NewStore(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to build gotime store: %w", err)
	}

	logger = logger.With().Str("context", "gotime_status_poller").Logger()

	cid := gotimeChannelID
	if shadowMode {
		cid = gotimeGopherdevChannelID
	}

	ln := logger.With().Str("context", "gotimestatus_notifier").Logger()
	gp, err := gotimestatus.New(gs, newHTTPClient(), logger, 30*time.Minute, goTimeStatusNotifyFactory(ln, sc, cid, shadowMode))
	if err != nil {
		return nil, fmt.Errorf("failed to create new gotime poller: %w", err)
	}

	t := time.NewTimer(0)
	w := make(chan struct{})

	go func() {
		defer close(w)
		logger.Info().Msg("starting GoTime poller")

		for {
			select {
			case <-t.C:
				gctx, cancel := context.WithTimeout(ctx, 10*time.Second)

				err := gp.Poll(gctx)

				cancel()

				t.Reset(5 * time.Minute)

				if err != nil {
					logger.Error().
						Err(err).
						Msg("trying GoTimeStatus poll again in 5 minutes")

					continue
				}

				logger.Trace().
					Msg("polling GoTimeStatus in 5 minutes")

			case <-ctx.Done():
				logger.Info().
					Err(ctx.Err()).
					Msg("context canceled: shutting down poller")

				return
			}
		}
	}()

	return w, nil
}
