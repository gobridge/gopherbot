// Package consumer provides a simple logging consumer of messages.
package consumer

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack/slackevents"
	"github.com/theckman/gopher2/config"
	"github.com/theckman/gopher2/workqueue"
)

type server struct {
	l *zerolog.Logger
	q workqueue.Q
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

	// set up the workqueue
	q, err := workqueue.New(workqueue.Config{
		ConsumerName:      cfg.Heroku.DynoID,
		ConsumerGroup:     cfg.Heroku.AppName,
		VisibilityTimeout: 10 * time.Second,
		RedisOptions:      redisOpts,
		Logger:            &logger,
	})
	if err != nil {
		return fmt.Errorf("failed to build workqueue: %w", err)
	}

	handler := func(ctx context.Context, m *slackevents.MessageEvent) (noAck bool, err error) {
		logger.Debug().
			Interface("json", m).
			Msg("message received")

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
