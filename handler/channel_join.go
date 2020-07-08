package handler

import (
	"fmt"
	"strings"

	"github.com/gobridge/gopherbot/mparser"
	"github.com/gobridge/gopherbot/workqueue"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack/slackevents"
)

// ChannelJoiner is the interface to represent an incoming team join event.
type ChannelJoiner interface {
	ChannelID() string
	UserID() string
}

type channelJoiner struct {
	channelID string
	userID    string
}

var _ ChannelJoiner = channelJoiner{}

func (j channelJoiner) ChannelID() string { return j.channelID }
func (j channelJoiner) UserID() string    { return j.userID }

// ChannelJoinActionFn is a function for handlers to take actions against team join
// events.
type ChannelJoinActionFn func(ctx workqueue.Context, tj ChannelJoiner, r Responder) error

type channelJoinAction struct {
	name string
	fn   ChannelJoinActionFn
}

// ChannelJoinActions represents actions to be taken on a team join event.
type ChannelJoinActions struct {
	shadow  bool
	actions map[string][]channelJoinAction
	l       zerolog.Logger
}

// NewChannelJoinActions returns a ChannelJoinActions for use.
func NewChannelJoinActions(shadowMode bool, l zerolog.Logger) *ChannelJoinActions {
	return &ChannelJoinActions{
		shadow:  shadowMode,
		actions: make(map[string][]channelJoinAction),
		l:       l,
	}
}

// Handler satisfies workqueue.ChannelJoinHandler.
func (c *ChannelJoinActions) Handler(ctx workqueue.Context, cj *slackevents.MemberJoinedChannelEvent) (bool, bool, error) {
	j := channelJoiner{
		channelID: cj.Channel,
		userID:    cj.User,
	}

	mention := mparser.Mention{
		Type: mparser.TypeUser,
		ID:   j.userID,
	}
	msg := NewMessage(j.channelID, cj.ChannelType, j.userID, "", "", "", "", nil)
	msg.allMentions = []mparser.Mention{mention}
	msg.userMentions = []mparser.Mention{mention}

	resp := response{
		sc: ctx.Slack(),
		m:  msg,
	}

	actions, ok := c.actions[j.channelID]
	if !ok {
		return false, true, nil // no reason given, as it's normal and shouldn't be logged
	}

	var someWorked bool

	for _, a := range actions {
		if c.shadow {
			c.l.Info().
				Str("channel_id", j.channelID).
				Str("user_id", j.userID).
				Bool("shadow_mode", true).
				Msg("would welcome user")
			continue
		}

		err := a.fn(ctx, j, resp)
		if err != nil {
			if someWorked {
				c.l.Error().
					Err(err).
					Str("channel_id", j.channelID).
					Str("user_id", j.userID).
					Str("join_action", a.name).
					Msg("failed to take action")

				return false, false, nil
			}

			// force a retry
			return true, false, fmt.Errorf("failed to take join action: %w", err)
		}

		someWorked = true
	}

	return false, false, nil
}

// Handle registers a ChannelJoinActionFn to be taken on new join events.
func (c *ChannelJoinActions) Handle(name, channelID string, fn ChannelJoinActionFn) {
	slice := c.actions[channelID]

	cja := channelJoinAction{
		name: name,
		fn:   fn,
	}

	slice = append(slice, cja)

	c.actions[channelID] = slice
}

// HandleStatic registers a ChannelJoinActionFn that sends an ephemeral message
// to the joining user. The message is the content variadic, joined by newlines.
func (c *ChannelJoinActions) HandleStatic(name, channelID string, content ...string) {
	m := strings.Join(content, "\n")

	fn := func(ctx workqueue.Context, cj ChannelJoiner, r Responder) error {
		return r.RespondEphemeral(ctx, m)
	}

	c.Handle(name, channelID, fn)
}
