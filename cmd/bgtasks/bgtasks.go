package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/go-redis/redis"
	"github.com/gobridge/gopherbot/config"
	"github.com/gobridge/gopherbot/internal/heartbeat"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

// runServer starts the gateway HTTP server.
func runServer(cfg config.C, logger zerolog.Logger) error {
	// set up signal catching
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	logger.Info().
		Str("env", string(cfg.Env)).
		Str("app", cfg.Heroku.AppName).
		Str("dyno_id", cfg.Heroku.DynoID).
		Str("commit", cfg.Heroku.Commit).
		Str("slack_client_id", cfg.Slack.ClientID).
		Str("log_level", cfg.LogLevel.String()).
		Msg("configuration values")

	rc := redis.NewClient(config.DefaultRedis(cfg))
	defer func() { _ = rc.Close() }()

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel() // only to appease govet

	lhb := logger.With().Str("context", "heartbeater").Logger()

	// start checking Redis health
	_, err := heartbeat.New(ctx, heartbeat.Config{
		RedisClient: rc,
		Logger:      lhb,
		AppName:     cfg.Heroku.AppName,
		UID:         cfg.Heroku.DynoID,
		Warn:        4 * time.Second,
		Fail:        8 * time.Second,
	})
	if err != nil {
		// maybe Redis is undergoing some maintenance
		// let's pause for a bit
		logger.Error().
			Err(err).
			Msg("failed to start heartbeating; sleeping for 10 seconds before exiting")

		time.Sleep(10 * time.Second)

		return fmt.Errorf("failed to heartbeat: %w", err)
	}

	sc := slack.New(cfg.Slack.BotAccessToken, slack.OptionHTTPClient(newHTTPClient()))

	var shadowMode bool
	if cfg.Env != config.Production {
		shadowMode = true
	}

	gerritDone, err := setUpGerrit(ctx, shadowMode, logger, sc, rc)
	if err != nil {
		return err
	}

	gotimeDone, err := setUpGoTime(ctx, shadowMode, logger, sc, rc)
	if err != nil {
		return err
	}

	ccDone, err := setUpChannelCacheFiller(ctx, logger, sc, rc)
	if err != nil {
		return err
	}

	// signal handling / graceful shutdown goroutine
	go func() {
		sig := <-signalCh

		cancel()

		logger.Info().
			Str("signal", sig.String()).
			Msg("shutting down bgtasks gracefully")
	}()

	logger.Info().Msg("presumably running...")
	<-gerritDone
	<-gotimeDone
	<-ccDone

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
