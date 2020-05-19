// Package consumer provides a simple logging consumer of messages.
package consumer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/go-redis/redis"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/theckman/gopher2/config"
	"github.com/theckman/gopher2/interactivity"
	"github.com/theckman/gopher2/mparser"
	"github.com/theckman/gopher2/workqueue"
)

func onlyOtherUserMMentions(selfID string, mentions []mparser.Mention) ([]mparser.Mention, bool) {
	if len(mentions) == 0 {
		return nil, false
	}

	var self bool

	fm := make([]mparser.Mention, 0, len(mentions)-1)

	for _, m := range mentions {
		if m.Type != mparser.TypeUser {
			continue
		}

		if m.ID == selfID {
			self = true
			continue
		}

		fm = append(fm, m)
	}

	return fm, self
}

func getSelf(c *slack.Client) (*slack.User, error) {
	// full lifetime of this function
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	at, err := c.AuthTestContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack authentication test failed: %w", err)
	}

	self, err := c.GetUserInfoContext(ctx, at.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to slack self user info: %w", err)
	}

	return self, nil
}

// RunServer starts the gateway HTTP server.
func RunServer(cfg config.C) error {
	// set up signal catching
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	// set up zerolog
	zerolog.TimestampFieldName = "timestamp"
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.SetGlobalLevel(cfg.LogLevel)

	// set up logging
	logger := zerolog.New(os.Stdout).
		With().Timestamp().Logger()

	logger.Info().
		Str("app", cfg.Heroku.AppName).
		Str("dyno_id", cfg.Heroku.DynoID).
		Str("commit", cfg.Heroku.Commit).
		Str("slack_request_token", cfg.Slack.RequestToken).
		Str("slack_client_id", cfg.Slack.ClientID).
		Str("log_level", cfg.LogLevel.String()).
		Msg("configuration values")

	// get redis config ready
	redisOpts := &redis.Options{
		Network:      "tcp",
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  11 * time.Second,
		WriteTimeout: 5 * time.Second,
		PoolSize:     20,
		MinIdleConns: 5,
		PoolTimeout:  5 * time.Second,
	}

	// if Redis is TLS secured
	if !cfg.Redis.Insecure {
		redisOpts.TLSConfig = &tls.Config{
			InsecureSkipVerify: cfg.Redis.SkipVerify,
		} // #nosec G402 -- Heroku Redis has an untrusted cert
	}

	sc := slack.New(cfg.Slack.BotAccessToken, slack.OptionHTTPClient(newHTTPClient()))

	// test credentails and get self reference
	self, err := getSelf(sc)
	if err != nil {
		return err
	}

	// set up the workqueue
	q, err := workqueue.New(workqueue.Config{
		ConsumerName:      cfg.Heroku.DynoID,
		ConsumerGroup:     cfg.Heroku.AppName,
		VisibilityTimeout: 10 * time.Second,
		RedisOptions:      redisOpts,
		Logger:            &logger,
		SlackClient:       sc,
		SlackUser:         self,
	})
	if err != nil {
		return fmt.Errorf("failed to build workqueue: %w", err)
	}

	ma := interactivity.New()

	handler := func(ctx workqueue.Context, m *slackevents.MessageEvent) (noAck bool, err error) {
		text, mentions := mparser.ParseAndSplice(m.Text, m.Channel)
		mentions, mentioned := onlyOtherUserMMentions(self.ID, mentions)

		actions := ma.Match(text, mentioned)

		for _, a := range actions {
			ctx.Logger().Debug().
				Str("action", a.Self).
				Msg("would taking action")

			/*
				err := a.Fn(ctx, m.Channel, m.TimeStamp, mentions, text)
				if err != nil {
					ctx.Logger().Error().
						Err(err).
						Str("action_description", a.Description).
						Msg("failed to take action")
				}
			*/
		}

		ctx.Logger().Debug().
			Str("text", m.Text).
			Int("actions", len(actions)).
			Int("mentions", len(mentions)).
			Msg("message handled")

		return false, nil
	}

	q.RegisterPublicMessageHandler(10*time.Second, handler)
	q.RegisterPrivateMessageHandler(10*time.Second, handler)

	// signal handling / graceful shutdown goroutine
	go func() {
		sig := <-signalCh

		logger.Info().
			Str("signal", sig.String()).
			Msg("shutting HTTP server down gracefully")

		q.Shutdown()
	}()

	logger.Info().Msg("waiting for events")

	q.Run()

	return nil
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: newHTTPTransport(),
	}
}

// newHTTPTransport returns an *http.Transport with some reasonable defaults.
func newHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
	}
}
