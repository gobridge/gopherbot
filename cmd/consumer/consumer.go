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
	"github.com/gobridge/gopherbot/cache"
	"github.com/gobridge/gopherbot/cmd/consumer/playground"
	"github.com/gobridge/gopherbot/config"
	"github.com/gobridge/gopherbot/glossary"
	"github.com/gobridge/gopherbot/handler"
	"github.com/gobridge/gopherbot/internal/heartbeat"
	"github.com/gobridge/gopherbot/workqueue"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

// playgroundChannelBlacklist sets a list of channels the playground uploader will
// not operate in
var playgroundChannelBlacklist = []string{
	"C4U9J9QBT", // #admin-help
	"C029RQSEG", // #random
	"G1L7RN06B", // admin private channel
	"G207C8R1R", // gobridge ops chanel
	"GB1KBRGKA", // modnar (private random channel)
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

func runServer(cfg config.C, logger zerolog.Logger) error {
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

	sc := slack.New(cfg.Slack.BotAccessToken, slack.OptionHTTPClient(newHTTPClient()))

	// test credentails and get self reference
	self, err := getSelf(sc)
	if err != nil {
		return err
	}

	rc := redis.NewClient(config.DefaultRedis(cfg))
	defer func() { _ = rc.Close() }()

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	lhb := logger.With().Str("context", "heartbeater").Logger()

	// start checking Redis health
	_, err = heartbeat.New(ctx, heartbeat.Config{
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

	cCache := cache.NewChannel(rc)

	// set up the workqueue
	q, err := workqueue.New(workqueue.Config{
		ConsumerName:      cfg.Heroku.DynoID,
		ConsumerGroup:     cfg.Heroku.AppName,
		VisibilityTimeout: 10 * time.Second,
		RedisClient:       rc,
		Logger:            &logger,
		SlackClient:       sc,
		SlackUser:         self,
		ChannelCache:      cCache,
	})
	if err != nil {
		return fmt.Errorf("failed to build workqueue: %w", err)
	}

	var shadowMode bool
	if cfg.Env != config.Production {
		shadowMode = true
	}

	ma, err := handler.NewMessageActions(
		self.ID,
		shadowMode,
		logger.With().Str("context", "message_actions").Logger(),
	)
	if err != nil {
		return fmt.Errorf("failed to build MessageActions handler: %w", err)
	}

	gloss := glossary.New(glossary.Prefix)

	tja := handler.NewTeamJoinActions(
		shadowMode,
		logger.With().Str("context", "team_join_actions").Logger(),
	)

	cja := handler.NewChannelJoinActions(
		shadowMode,
		logger.With().Str("context", "channel_join_actions").Logger(),
	)

	// set up all the responders and reacters
	injectMessageResponses(ma)
	injectMessageResponseFuncs(ma)
	injectMessageReactions(ma)
	injectMessageResponsePrefix(ma)

	// handle "define " prefixed command
	ma.HandlePrefix(glossary.Prefix, "find a definition in the glossary of Go-related terms", gloss.DefineHandler)

	// set up the Go Playground uploader
	lp := logger.With().Str("context", "playground")
	pg := playground.New(newHTTPClient(), lp.Logger(), playgroundChannelBlacklist)
	ma.HandleDynamic(pg.MessageMatchFn, pg.Handler)

	injectTeamJoinHandlers(tja)
	injectChannelJoinHandlers(cja)

	q.RegisterTeamJoinsHandler(2*time.Second, tja.Handler)
	q.RegisterChannelJoinsHandler(10*time.Second, cja.Handler)
	q.RegisterPublicMessagesHandler(10*time.Second, ma.Handler)
	q.RegisterPrivateMessagesHandler(10*time.Second, ma.Handler)

	// signal handling / graceful shutdown goroutine
	go func() {
		sig := <-signalCh

		logger.Info().
			Str("signal", sig.String()).
			Msg("shutting down consumer gracefully")

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
