package workqueue

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

// ChannelSvc is an interface providing the channel service.
type ChannelSvc interface {
	Lookup(channelName string) (slack.Channel, bool, error)
}

// EventMetadata represents the metadata about the event
type EventMetadata struct {
	// ID represents the ID as given to us by Slack.
	ID string

	// Time is the time the event was emitted according to Slack.
	Time time.Time

	// IngestTime is the time we received the event from Slack.
	IngestTime time.Time

	// RedisEvent is the ID of the message sent through the Redis queue.
	RedisEvent string
}

// Context is a superset of context.Context, including methods needed by
// workqueue handler authors. The context given to handlers has a timeout for when they should
type Context interface {
	context.Context

	// Meta is the event metadata about this job. When did Slack emit it, when
	// did we ingest it, etc.
	Meta() EventMetadata

	// Logger presents the logger create for this handler. This logger isn't
	// global, and is instead request local
	Logger() *zerolog.Logger

	// Slack is the Slack client.
	Slack() *slack.Client

	// Self is the info for the bot user we're using the credentials of.
	Self() slack.User

	// ChannelSvc provides a way to work with the internal channel metadata
	// cache.
	ChannelSvc() ChannelSvc
}

type ctxer struct {
	context.Context

	s *slack.Client
	l *zerolog.Logger
	u *slack.User
	c ChannelSvc
	e EventMetadata
}

// Meta satisfies Context.
func (c ctxer) Meta() EventMetadata {
	return c.e
}

// Slack satisfies Context.
func (c ctxer) Slack() *slack.Client {
	return c.s
}

// Logger satisfies Context.
func (c ctxer) Logger() *zerolog.Logger {
	return c.l
}

// Self satisfies Context.
func (c ctxer) Self() slack.User {
	return *c.u
}

// ChannelSvc satisfies Context.
func (c ctxer) ChannelSvc() ChannelSvc {
	return c.c
}

var _ Context = ctxer{}
