// Package gateway provides the gopher gateway server
package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis"
	"github.com/gobridge/gopherbot/config"
	"github.com/gobridge/gopherbot/internal/heartbeat"
	"github.com/gobridge/gopherbot/workqueue"
	"github.com/rs/zerolog"
)

// RunServer starts the gateway HTTP server.
func RunServer(cfg config.C, logger zerolog.Logger) error {
	// set up signal catching
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	logger.Info().
		Str("env", string(cfg.Env)).
		Str("app", cfg.Heroku.AppName).
		Str("dyno_id", cfg.Heroku.DynoID).
		Str("commit", cfg.Heroku.Commit).
		Str("slack_request_token", cfg.Slack.RequestToken).
		Str("slack_client_id", cfg.Slack.ClientID).
		Str("log_level", cfg.LogLevel.String()).
		Msg("configuration values")

	rc := redis.NewClient(config.DefaultRedis(cfg))
	defer func() { _ = rc.Close() }()

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

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

	// set up the workqueue
	q, err := workqueue.New(workqueue.Config{
		ConsumerName:      cfg.Heroku.DynoID,
		ConsumerGroup:     cfg.Heroku.AppName,
		VisibilityTimeout: 10 * time.Second,
		RedisClient:       rc,
		Logger:            &logger,
	})
	if err != nil {
		return fmt.Errorf("failed to build workqueue: %w", err)
	}

	// set up the handler
	hnd := handler{
		l: &logger,
		q: q,
	}

	// set up the router
	mux := http.NewServeMux()
	mux.HandleFunc("/", hnd.handleNotFound)
	mux.HandleFunc("/_ruok", hnd.handleRUOK)

	// wrap our slack event handler in the slackSignature middleware.
	// wrap the slackSignature middleware in the context / heroku header middleware
	slackHandler := chMiddlewareFactory(
		logger,
		slackSignatureMiddlewareFactory(
			cfg.Slack.RequestSecret, cfg.Slack.RequestToken, cfg.Slack.AppID, cfg.Slack.TeamID, &logger, hnd.handleSlackEvent,
		),
	)

	mux.HandleFunc("/slack/event", slackHandler)

	socketAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	logger.Info().
		Str("addr", socketAddr).
		Msg("binding to TCP socket")

	// set up the network socket
	listener, err := net.Listen("tcp", socketAddr)
	if err != nil {
		return fmt.Errorf("failed to open HTTP socket: %w", err)
	}

	defer func() { _ = listener.Close() }()

	// set up the HTTP server
	httpSrvr := &http.Server{
		Handler:     mux,
		ReadTimeout: 20 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	serveStop, serverShutdown := make(chan struct{}), make(chan struct{})
	var serveErr, shutdownErr error

	// HTTP server parent goroutine
	go func() {
		defer close(serveStop)
		serveErr = httpSrvr.Serve(listener)
	}()

	// signal handling / graceful shutdown goroutine
	go func() {
		defer close(serverShutdown)
		sig := <-signalCh

		logger.Info().
			Str("signal", sig.String()).
			Msg("shutting HTTP server down gracefully")

		cctx, ccancel := context.WithTimeout(context.Background(), 25*time.Second)

		defer ccancel()
		defer cancel()

		if shutdownErr = httpSrvr.Shutdown(cctx); shutdownErr != nil {
			logger.Error().
				Err(shutdownErr).
				Msg("failed to gracefully shut down HTTP server")
		}
	}()

	// wait for it to die
	<-serverShutdown
	<-serveStop

	// log errors for informational purposes
	logger.Info().
		AnErr("serve_err", serveErr).
		AnErr("shutdown_err", shutdownErr).
		Msg("server shut down")

	return nil
}
