package workqueue

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

// Context is a superset of context.Context, including methods needed by
// workqueue handler authors. The context given to handlers has a timeout for when they should
type Context interface {
	context.Context

	// Logger presents the logger create for this handler. This logger isn't
	// global, and is instead request local
	Logger() *zerolog.Logger

	// Slack is the Slack client.
	Slack() *slack.Client

	// Self is the info for the bot user we're using the credentials of.
	Self() slack.User
}

type ctxer struct {
	context.Context

	s *slack.Client
	l *zerolog.Logger
	u *slack.User
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

var _ Context = ctxer{}
