package interactivity

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/theckman/gopher2/mparser"
	"github.com/theckman/gopher2/workqueue"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type reactiveAction struct {
	description string
	fn          ActionFn
}

// ActionFn represents an interactivity action where it's not a simple response,
// and instead needs a function execution.
type ActionFn func(ctx workqueue.Context, channelID, messageTS string, toMention []mparser.Mention, trigger string) error

// MessageActions is the central point to the interactivity package, and gives
// us a way to register handlers and then match those against inputs.
type MessageActions struct {
	responses       map[string]reactiveAction
	prefixResponses map[string]reactiveAction
	reactions       map[string]reactiveAction
}

// New returns a new MessageActions struct.
func New() *MessageActions {
	ra := &MessageActions{
		responses:       make(map[string]reactiveAction),
		prefixResponses: make(map[string]reactiveAction),
		reactions:       make(map[string]reactiveAction),
	}

	injectMessageResponses(ra)
	injectMessageResponseFuncs(ra)
	injectMessageReactions(ra)

	return ra
}

var messageTriggerAlias = map[string]string{
	"di":        "dependency injection",
	"coin flip": "flip a coin",
	"flip coin": "flip a coin",
}

// Action represents a single piece of interactive action to be taken.
type Action struct {
	Self        string
	Description string
	Fn          ActionFn
}

// Match looks at the trigger to see if it matches any known handlers. Some
// handlers are only invoked if the bot was mentioned.
func (r *MessageActions) Match(trigger string, mentioned bool) []Action {
	t := trigger
	if v, ok := messageTriggerAlias[trigger]; ok {
		t = v
	}

	var aa []Action

	for k, v := range r.reactions {
		if strings.Contains(t, k) {
			a := Action{
				Self:        k,
				Description: v.description,
				Fn:          v.fn,
			}
			aa = append(aa, a)
		}
	}

	for k, v := range r.prefixResponses {
		if strings.HasPrefix(t, k) {
			a := Action{
				Self:        k,
				Description: v.description,
				Fn:          v.fn,
			}
			aa = append(aa, a)
		}
	}

	if mentioned {
		for k, v := range r.responses {
			if t == k {
				a := Action{
					Self:        k,
					Description: v.description,
					Fn:          v.fn,
				}
				aa = append(aa, a)
			}
		}
	}

	return aa
}

// Handle is a simple response handler. If the trigger matches a known value,
// return the content. The bot must be mentioned for this to match.
func (r *MessageActions) Handle(trigger, description string, content ...string) {
	fn := func(ctx workqueue.Context, channelID, messageTS string, toMention []mparser.Mention, trigger string) error {
		c := mparser.Join(toMention, " ") + strings.Join(content, "\n")

		msg := slack.MsgOptionText(c, true)

		_, _, _, err := ctx.Slack().SendMessageContext(ctx, channelID, msg)

		return err
	}

	r.HandleFunc(trigger, description, fn)
}

// HandleFunc is similar to Handle(), but instead of specifying which content to
// reply with you provide your own function. The bot must be mentioned for this
// to match.
func (r *MessageActions) HandleFunc(trigger, description string, fn ActionFn) {
	if len(trigger) == 0 {
		panic("trigger cannot be empty string")
	}

	if fn == nil {
		panic("fn cannot be nil")
	}

	if _, ok := r.responses[trigger]; ok {
		panic(fmt.Sprintf("trigger %q already exists", trigger))
	}

	r.responses[trigger] = reactiveAction{
		description: description,
		fn:          fn,
	}
}

// HandleReaction handles reacting to messages that contain trigger anywhere in
// the message.
func (r *MessageActions) HandleReaction(trigger string, respond, random bool, reactions ...string) {
	if len(trigger) == 0 {
		panic("trigger cannot be empty string")
	}

	if len(reactions) == 0 {
		panic("reactions variadic cannot be empty")
	}

	if _, ok := r.responses[trigger]; ok {
		panic(fmt.Sprintf("trigger %q already exists", trigger))
	}

	r.reactions[trigger] = reactiveAction{
		fn: func(ctx workqueue.Context, channelID, messageTS string, _ []mparser.Mention, _ string) error {
			if random && rand.Intn(150) != 0x2A { // not this time, maybe next time!
				return nil
			}

			for _, r := range reactions {
				if !respond {
					if err := ctx.Slack().AddReactionContext(ctx, r, slack.ItemRef{Channel: channelID, Timestamp: messageTS}); err != nil {
						return fmt.Errorf("failed to react with %s: %w", r, err)
					}
				} else {
					_, _, _, err := ctx.Slack().SendMessageContext(ctx, channelID, slack.MsgOptionText(r, true))
					if err != nil {
						return fmt.Errorf("failed to respond-react: %w", err)
					}
				}
			}

			return nil
		},
	}
}

// HandlePrefix handles a message with any prefix, regardless of the mentions in
// it.
func (r *MessageActions) HandlePrefix(prefix, description string, fn ActionFn) {
	if len(prefix) == 0 {
		panic("prefix cannot be empty string")
	}

	if fn == nil {
		panic("fn cannot be nil")
	}

	if _, ok := r.prefixResponses[prefix]; ok {
		panic(fmt.Sprintf("prefix %q already exists", prefix))
	}

	r.prefixResponses[prefix] = reactiveAction{
		description: description,
		fn:          fn,
	}
}
