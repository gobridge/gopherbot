package workqueue

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

type Context interface {
	context.Context

	Slack() *slack.Client
	Logger() *zerolog.Logger
}

type ctxer struct {
	context.Context

	s *slack.Client
	l *zerolog.Logger
}

// Slack satisfies Context.
func (c ctxer) Slack() *slack.Client {
	return c.s
}

// Logger satisfies Context.
func (c ctxer) Logger() *zerolog.Logger {
	return c.l
}

var _ Context = ctxer{}
