package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/gobridge/gopherbot/internal/poller/gerrit"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

const (
	gerritGopherdevChannelID = "C013XC5SU21"
	gerritGolangclsChannelID = "C2VU4UTFZ"
)

func gerritNotifyFactory(logger zerolog.Logger, c *slack.Client, channelID string, shadowMode bool) gerrit.NotifyFunc {
	return func(ctx context.Context, cl gerrit.CL) error {
		if shadowMode {
			logger.Info().
				Bool("shadow_mode", true).
				Msg("would announce merged CL")

			return nil
		}

		msg := fmt.Sprintf("[%d] %s: %s", cl.Number, cl.Message(), cl.Link())

		a := slack.Attachment{
			Title:     cl.Subject,
			TitleLink: cl.Link(),
			Text:      cl.Revisions[cl.CurrentRevision].Commit.Message,
			Footer:    cl.ChangeID,
		}

		opts := []slack.MsgOption{
			slack.MsgOptionDisableLinkUnfurl(),
			slack.MsgOptionText(msg, false),
			slack.MsgOptionAttachments(a),
		}

		_, _, _, err := c.SendMessageContext(ctx, channelID, opts...)

		return err
	}
}

const gerritPollTimeKey = "bgtasks:poller:gerrit:last_refresh_ts"

func lastPoll(rc *redis.Client) (time.Time, error) {
	res := rc.Get(gerritPollTimeKey)
	if err := res.Err(); err != nil {
		if err == redis.Nil {
			return time.Time{}, nil
		}

		return time.Time{}, fmt.Errorf("failed to get key from redis: %w", err)
	}

	ts, err := res.Int64()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return time.Unix(ts, 0), nil
}

func updateLastPoll(rc *redis.Client) error {
	now := time.Now().UnixNano() / int64(time.Second)

	res := rc.Set(gerritPollTimeKey, now, 31*24*time.Hour)
	if err := res.Err(); err != nil {
		return fmt.Errorf("failed to set poll time: %w", err)
	}

	return nil
}

func initialTimer(last time.Time, duration time.Duration) time.Duration {
	tu := time.Until(last.Add(duration))
	if tu < 0 {
		tu = 0
	}
	return tu
}

func setUpGerrit(ctx context.Context, shadowMode bool, logger zerolog.Logger, sc *slack.Client, rc *redis.Client) (chan struct{}, error) {
	gs, err := gerrit.NewStore(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to build gerrit store: %w", err)
	}

	logger = logger.With().Str("context", "gerrit_poller").Logger()

	hr := 10 * time.Minute  // healthy refresh duration
	uhr := 10 * time.Minute // unhealthy refresh duration
	cid := gerritGolangclsChannelID

	if shadowMode {
		hr = 60 * time.Minute
		cid = gerritGopherdevChannelID
	}

	ln := logger.With().Str("context", "gerrit_notifier").Logger()
	gp, err := gerrit.New(gs, newHTTPClient(), logger, gerritNotifyFactory(ln, sc, cid, shadowMode))
	if err != nil {
		return nil, fmt.Errorf("failed to create new gerrit poller: %w", err)
	}

	lp, err := lastPoll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to get last gerrit poll time: %w", err)
	}

	initialDur := initialTimer(lp, hr)

	logger.Info().
		Str("timer_duration", initialDur.String()).
		Msg("setting gerrit poll timer")

	t := time.NewTimer(initialDur)
	w := make(chan struct{})

	go func() {
		for {
			select {
			case <-t.C:
				gctx, cancel := context.WithTimeout(ctx, 10*time.Second)

				err := gp.Poll(gctx)

				cancel()

				if err != nil {
					logger.Error().
						Err(err).
						Str("timer_duration", uhr.String()).
						Msg("trying gerrit poll again after timer fires")

					t.Reset(uhr)

					continue
				}

				t.Reset(hr)

				if err = updateLastPoll(rc); err != nil {
					logger.Error().
						Err(err).
						Msg("failed to save latest poll time")
				}

				logger.Info().
					Str("timer_duration", hr.String()).
					Msg("resetting gerrit poll timer")

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
