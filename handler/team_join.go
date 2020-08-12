package handler

import (
	"fmt"
	"time"

	"github.com/gobridge/gopherbot/mparser"
	"github.com/gobridge/gopherbot/workqueue"
	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
)

// TeamJoiner is the interface to represent an incoming team join event.
type TeamJoiner interface {
	User() slack.User
}

type teamJoiner slack.User

func (j teamJoiner) User() slack.User { return slack.User(j) }

// TeamJoinActionFn is a function for handlers to take actions against team join
// events.
type TeamJoinActionFn func(ctx workqueue.Context, tj TeamJoiner, r Responder) error

type teamJoinAction struct {
	name string
	fn   TeamJoinActionFn
}

// TeamJoinActions represents actions to be taken on a team join event.
type TeamJoinActions struct {
	shadow  bool
	actions []teamJoinAction
	l       zerolog.Logger
}

// NewTeamJoinActions returns a TeamJoinActions for use.
func NewTeamJoinActions(shadowMode bool, l zerolog.Logger) *TeamJoinActions {
	return &TeamJoinActions{shadow: shadowMode, l: l}
}

// Handler satisfies workqueue.TeamJoinHandler.
func (t *TeamJoinActions) Handler(ctx workqueue.Context, tj *slack.TeamJoinEvent) (bool, bool, error) {
	j := teamJoiner(tj.User)

	mention := mparser.Mention{
		Type: mparser.TypeUser,
		ID:   j.ID,
	}
	msg := NewMessage(j.ID, "im", j.ID, "", "", "", "", nil)
	msg.allMentions = []mparser.Mention{mention}
	msg.userMentions = []mparser.Mention{mention}

	resp := response{
		sc: ctx.Slack(),
		m:  msg,
	}

	var someWorked bool

	for _, a := range t.actions {
		if t.shadow {
			t.l.Info().
				Str("user_id", tj.User.ID).
				Bool("shadow_mode", true).
				Msg("would welcome user")
			continue
		}

		err := a.fn(ctx, j, resp)
		if err != nil {
			if someWorked {
				t.l.Error().
					Err(err).
					Str("join_action", a.name).
					Msg("failed to take action")

				return false, false, nil
			}

			// if it's too old discard
			if time.Since(ctx.Meta().Time) >= 10*time.Minute {
				return false, true, fmt.Errorf("discarding failed join action due to age: %w", err)
			}

			// force a retry
			return true, false, fmt.Errorf("failed to take join action: %w", err)
		}

		someWorked = true
	}

	return false, false, nil
}

// Handle registers a TeamJoinActionFn to be taken on new join events.
func (t *TeamJoinActions) Handle(name string, fn TeamJoinActionFn) {
	t.actions = append(t.actions, teamJoinAction{name, fn})
}
