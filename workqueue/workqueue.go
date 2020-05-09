package workqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/robinjoseph08/redisqueue"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

// Event matches external event types to the Redis stream names we're using
type Event string

const (
	slackPublicMessage  = "slack_message_public"
	slackPrivateMessage = "slack_message_private"
	slackTeamJoin       = "slack_team_join"
	slackChannelJoin    = "slack_channel_join"
)

const (
	// SlackMessageChannel is the Event for a message with a channel_type of "channel"
	SlackMessageChannel Event = slackPublicMessage

	// SlackMessageAppHome is the Event for a message with a channel_type of "app_home"
	SlackMessageAppHome Event = slackPrivateMessage

	// SlackMessageGroup is the Event for a message with a channel_type of "group",
	// aka a private channel
	SlackMessageGroup Event = slackPrivateMessage

	// SlackMessageIM is the Event for a message with a channel_type of "im",
	// aka a DM
	SlackMessageIM Event = slackPrivateMessage

	// SlackMessageMPIM is the Event for a message with a channel_type "mpim",
	// aka a group DM
	SlackMessageMPIM Event = slackPrivateMessage

	// SlackTeamJoin is the Event for a team (workspace) join Slack event
	SlackTeamJoin Event = slackTeamJoin

	// SlackChannelJoin is the Event for a channel (public or private) join Slack event.
	SlackChannelJoin Event = slackChannelJoin
)

// MessageHandler is the handler for public Slack messages. The handler
// signals to the workqueue what to do with the item on failure with the noAck
// bool. If there is an error, and noAck is true, another worker should pick up
// the work eventually (assuming there are others).
type MessageHandler func(ctx context.Context, m *slackevents.MessageEvent) (noAck bool, err error)

// TeamJoinHandler is the handler for team_join Slacfk events, used when a new
// member joins the workspace. For info on noAck please see the comment for the
// MessageHandler type.
type TeamJoinHandler func(ctx context.Context, t *slack.TeamJoinEvent) (noAck bool, err error)

// Publisher is the interface for the workqueue publish behavior.
type Publisher interface {
	Publish(e Event, eventTimestamp int64, eventID, requetID string, jsonData []byte) error
}

// Registerer is the interface for handler registrations within the workqueue.
type Registerer interface {
	RegisterTeamJoinHandler(timeout time.Duration, fn TeamJoinHandler)
	RegisterPublicMessageHandler(timeout time.Duration, fn MessageHandler)
	RegisterPrivateMessageHandler(timeout time.Duration, fn MessageHandler)
}

// Q is an interface to describe the entirety of the workqueue.
type Q interface {
	Publisher
	Registerer
}

// Config is the I configuration
type Config struct {
	// ConsumerName is this node's unique identifier. Leave blank to use
	// hostname.
	ConsumerName string

	// ConsumerGroup is likely this node's application or service name. Leave
	// blank to use hostname, although that's not recommended. If you are only
	// producing events this is safe to be kept blank.
	ConsumerGroup string

	// VisibilityTimeout is how long a consumer will wait for others to finish a
	// task before assuming they are dead and stealing it. If you're acting as
	// only a producer this can be left as its zero value.
	VisibilityTimeout time.Duration

	// RedisOptions are what they say on the tin.
	RedisOptions *redis.Options

	// Logger is the logger
	Logger *zerolog.Logger
}

// I is the workqueue struct, which satisfies Q.
type I struct {
	p *redisqueue.Producer
	c *redisqueue.Consumer

	l *zerolog.Logger
}

// compile time check: does *I satisfy Q?
var _ Q = (*I)(nil)

// New returns a new *I or an error. The consumerName, consumerGroup, and
// visibilityTimeout can be left at their zero value if you're only using I to
// publish.
func New(cfg Config) (*I, error) {
	p, err := redisqueue.NewProducerWithOptions(&redisqueue.ProducerOptions{
		ApproximateMaxLength: true,
		StreamMaxLength:      1024,
		RedisOptions:         cfg.RedisOptions,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to make producer: %w", err)
	}

	c, err := redisqueue.NewConsumerWithOptions(&redisqueue.ConsumerOptions{
		Name:              cfg.ConsumerName,
		GroupName:         cfg.ConsumerGroup,
		VisibilityTimeout: cfg.VisibilityTimeout,
		BlockingTimeout:   10 * time.Second,
		ReclaimInterval:   time.Second,
		BufferSize:        1,
		Concurrency:       2,
		RedisOptions:      cfg.RedisOptions,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare consumer: %w", err)
	}

	i := &I{
		p: p,
		c: c,
		l: cfg.Logger,
	}

	return i, nil
}

// Run wraps the redisqueue.Consumer.Run method
func (i *I) Run() {
	i.c.Run()
}

// SHutdown wraps the redisqueue.Consumer.Shutdown method
func (i *I) Shutdown() {
	i.c.Shutdown()
}

// Publish takes an Event, which roughly map to different Slack event types, the event timestamp (from the Slack side),
func (i *I) Publish(e Event, eventTimestamp int64, eventID, requestID string, jsonData []byte) error {
	return i.p.Enqueue(&redisqueue.Message{
		Stream: string(e),
		Values: map[string]interface{}{
			"request_id": requestID,
			"gateway_ts": strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10),
			"event_ts":   strconv.FormatInt(eventTimestamp, 10),
			"event_id":   eventID,
			"json":       string(jsonData),
		},
	})
}

// RegisterPublicMessageHandler is the method to register a new handler for
// public Slack messages. That would be those sent to a public channel. The
// timeout argument specifies how long the handler has to complete, before its
// context is canceled.
func (i *I) RegisterPublicMessageHandler(timeout time.Duration, fn MessageHandler) {
	i.registerMessageHandler(slackPublicMessage, timeout, fn)
}

// RegisterPrivateMessageHandler is the method to register a new handler for
// private Slack messages. This would be those sent to a private channel, a
// 1-on-1 DM, or a group DM. The timeout argument specifies how long the handler
// has to complete, before its context is canceled.
func (i *I) RegisterPrivateMessageHandler(timeout time.Duration, fn MessageHandler) {
	i.registerMessageHandler(slackPrivateMessage, timeout, fn)
}

func (i *I) registerMessageHandler(stream string, timeout time.Duration, fn MessageHandler) {
	i.c.RegisterWithLastID(stream, "$", messageHandlerFactory(i.l, timeout, fn))
}

// RegisterTeamJoinHandler registers the handler for events related to people
// joining the Slack workspace.
func (i *I) RegisterTeamJoinHandler(timeout time.Duration, fn TeamJoinHandler) {
	i.c.RegisterWithLastID(slackTeamJoin, "$", teamJoinHandlerFactory(i.l, timeout, fn))
}

func messageHandlerFactory(baseLogger *zerolog.Logger, timeout time.Duration, fn MessageHandler) redisqueue.ConsumerFunc {
	flogger := baseLogger.With().Str("handler", "message").Logger()

	return func(m *redisqueue.Message) error {
		start := time.Now()

		// build message-local logging context
		logger := flogger.With().
			Str("redis_message", m.ID).
			Str("redis_stream", m.Stream).
			Logger()

		eid, et, gt, d, err := parseGatewayMessage(m)
		if err != nil {
			logger.Error().
				Err(err).
				TimeDiff("duration", time.Now(), start).
				Msg("failed to parse message from gateway")

			return nil
		}

		// log time fired on Slack side, and time it was enqueued
		logger = logger.With().
			Time("event_time", et).
			Str("event_id", eid).
			Time("enqueued_time", gt).Logger()

		var sm *slackevents.MessageEvent

		err = json.Unmarshal([]byte(d), &sm)
		if err != nil {
			logger.Error().
				Err(err).
				TimeDiff("duration", time.Now(), start).
				Msg("failed to parse message JSON")

			// we can't process it
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		// used to calculate handler duration
		bht := time.Now()

		noAck, err := fn(ctx, sm)

		// handler runtime duration
		hrd := time.Since(bht)

		cancel()

		logger = logger.With().Dur("handler_duration", hrd).Logger()

		if err != nil {
			logger.Error().Err(err).
				Bool("no_ack", noAck).
				TimeDiff("duration", time.Now(), start).
				Msg("handler failed")

			if noAck {
				return err
			}

			return nil
		}

		logger.Info().
			TimeDiff("duration", time.Now(), start).
			Msg("complete")

		return nil
	}
}

func teamJoinHandlerFactory(baseLogger *zerolog.Logger, timeout time.Duration, fn TeamJoinHandler) redisqueue.ConsumerFunc {
	flogger := baseLogger.With().Str("handler", "team_join").Logger()

	return func(m *redisqueue.Message) error {
		start := time.Now()

		// build message-local logging context
		logger := flogger.With().
			Str("redis_message", m.ID).
			Str("redis_stream", m.Stream).
			Logger()

		eid, et, gt, d, err := parseGatewayMessage(m)
		if err != nil {
			logger.Error().
				Err(err).
				TimeDiff("duration", time.Now(), start).
				Msg("failed to parse message from gateway")

			return nil
		}

		// log time fired on Slack side, and time it was enqueued
		logger = logger.With().
			Time("event_time", et).
			Str("event_id", eid).
			Time("enqueued_time", gt).Logger()

		var stj *slack.TeamJoinEvent

		if err := json.Unmarshal([]byte(d), &stj); err != nil {
			logger.Error().
				Err(err).
				TimeDiff("duration", time.Now(), start).
				Msg("failed to parse message JSON")

			// we can't process it
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		// used to calculate handler duration
		bht := time.Now()

		noAck, err := fn(ctx, stj)

		// handler runtime duration
		hrd := time.Since(bht)

		cancel()

		logger = logger.With().Dur("handler_duration", hrd).Logger()

		if err != nil {
			logger.Error().Err(err).
				Bool("no_ack", noAck).
				TimeDiff("duration", time.Now(), start).
				Msg("handler failed")

			if noAck {
				return err
			}

			return nil
		}

		logger.Info().
			TimeDiff("duration", time.Now(), start).
			Msg("complete")

		return nil
	}
}

func unix(i int64) (int64, int64) {
	// convert milliseconds to whole seconds
	// convert millisecond remainder from above conversion to nanoseconds
	return i / 1000, (i % 1000) * int64(time.Millisecond)
}

func parseGatewayMessage(m *redisqueue.Message) (eventID string, eventTime, gatewayTime time.Time, data string, err error) {
	eti, ok := m.Values["event_ts"]
	if !ok {
		return "", time.Time{}, time.Time{}, "", errors.New("redis stream malformed: event_ts not present")
	}

	gti, ok := m.Values["gateway_ts"]
	if !ok {
		return "", time.Time{}, time.Time{}, "", errors.New("redis stream malformed: gateway_ts not present")
	}

	eidi, ok := m.Values["event_id"]
	if !ok {
		return "", time.Time{}, time.Time{}, "", errors.New("redis stream malformed: event_id not present")
	}

	di, ok := m.Values["json"]
	if !ok {
		return "", time.Time{}, time.Time{}, "", errors.New("redis stream malformed: json data not present")
	}

	d, ok := di.(string)
	if !ok {
		return "", time.Time{}, time.Time{}, "", errors.New("json data is not a string")
	}

	eid, ok := eidi.(string)
	if !ok {
		return "", time.Time{}, time.Time{}, "", errors.New("event_id data is not a string")
	}

	ets, ok := eti.(string)
	if !ok {
		return "", time.Time{}, time.Time{}, "", errors.New("event_ts is not a string")
	}

	gts, ok := gti.(string)
	if !ok {
		return "", time.Time{}, time.Time{}, "", errors.New("gateway_ts is not a string")
	}

	et, err := strconv.ParseInt(ets, 10, 64)
	if err != nil {
		return "", time.Time{}, time.Time{}, "", fmt.Errorf("failed to parse event_ts %q: %w", ets, err)
	}

	gt, err := strconv.ParseInt(gts, 10, 64)
	if err != nil {
		return "", time.Time{}, time.Time{}, "", fmt.Errorf("failed to parse gateway_ts %q: %w", gts, err)
	}

	ett := time.Unix(et, 0)

	s, ns := unix(gt)
	gtt := time.Unix(s, ns)

	return eid, ett, gtt, d, nil
}
