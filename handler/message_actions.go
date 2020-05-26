package handler

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/slack-go/slack/slackevents"
	"github.com/gobridge/gopherbot/mparser"
	"github.com/gobridge/gopherbot/workqueue"
)

// MessageActionFn represents an interactivity action where it's not a simple response,
// and instead needs a function execution.
type MessageActionFn func(ctx workqueue.Context, m Messenger, r Responder) error

// MessageMatchFn is a function for consumers to provider their own handler match. If
// the MessageMatchFn returns true, the handler matches.
type MessageMatchFn func(m Messenger) bool

type reactiveAction struct {
	description       string
	onlyWhenMentioned bool
	aliases           []string
	fn                MessageActionFn
	matchfn           MessageMatchFn
}

// MessageAction represents a single piece of interactive action to be taken.
type MessageAction struct {
	Self        string
	Description string
	fn          MessageActionFn

	m Message
}

// Do is the MessageAction's enacter. It uses the Slack client from the
// workqueue.Context to for handler functions to use.
func (a MessageAction) Do(ctx workqueue.Context) error {
	r := response{
		sc: ctx.Slack(),
		m:  a.m,
	}

	return a.fn(ctx, a.m, r)
}

// RegisteredMessageHandler is what is returned from the MessageActions.Registered()
// method.
type RegisteredMessageHandler struct {
	Trigger     string
	Prefix      bool
	Description string
	Aliases     []string
}

// MessageActions is the central point to the interactivity package, and gives
// us a way to register handlers and then match those against inputs.
type MessageActions struct {
	responses       map[string]reactiveAction
	prefixResponses map[string]reactiveAction
	reactions       map[string]reactiveAction
	dynamic         []reactiveAction

	aliases map[string]string

	selfID     string
	shadowMode bool
	logger     zerolog.Logger
}

// NewMessageActions returns a new MessageActions struct.
func NewMessageActions(selfID string, shadowMode bool, logger zerolog.Logger) (*MessageActions, error) {
	if len(selfID) == 0 {
		return nil, errors.New("selfID must be set")
	}

	ma := &MessageActions{
		responses:       make(map[string]reactiveAction),
		prefixResponses: make(map[string]reactiveAction),
		reactions:       make(map[string]reactiveAction),
		aliases:         make(map[string]string),
		selfID:          selfID,
		shadowMode:      shadowMode,
		logger:          logger,
	}

	return ma, nil
}

// Registered returns a list of registered handlers. You could use this to build
// help output.
func (m *MessageActions) Registered() []RegisteredMessageHandler {
	rhs := make([]RegisteredMessageHandler, 0, len(m.responses)+len(m.prefixResponses))

	for k, v := range m.responses {
		rh := RegisteredMessageHandler{
			Trigger:     k,
			Description: v.description,
			Aliases:     v.aliases,
		}
		rhs = append(rhs, rh)
	}

	for k, v := range m.prefixResponses {
		rh := RegisteredMessageHandler{
			Trigger:     k,
			Prefix:      true,
			Description: v.description,
		}
		rhs = append(rhs, rh)
	}

	return rhs
}

func shouldDiscard(m *slackevents.MessageEvent) (string, bool) {
	if len(m.SubType) > 0 {
		return "message has subtype", true
	}

	tss := strings.Split(m.TimeStamp, ".")[0]

	// we assume this is a well-formed message
	// and if it isn't, this should be 0 and force a discard
	epoch, err := strconv.ParseInt(tss, 10, 64)
	if err != nil {
		return "timestamp malformed", true
	}

	// is the message too old?
	if time.Now().Unix()-epoch > 30 {
		return "message older than 30 seconds", true
	}

	return "", false
}

// Handler is the method that should satisfy a workqueue handler.
func (m *MessageActions) Handler(ctx workqueue.Context, me *slackevents.MessageEvent) (bool, bool, error) {
	if me.User == ctx.Self().ID {
		ctx.Logger().Debug().Msg("ignoring message from self")
		return false, false, nil // no reason given, as it's normal and shouldn't be logged
	}

	if reason, discard := shouldDiscard(me); discard {
		return false, true, fmt.Errorf("discarding message: %s", reason)
	}

	actions := m.Match(
		NewMessage(
			me.Channel, me.ChannelType, me.User, me.ThreadTimeStamp, me.TimeStamp, me.Text, me.Files,
		),
	)

	for _, a := range actions {
		ctx.Logger().Debug().
			Str("action", a.Self).
			Msg("taking action")

		err := a.Do(ctx)
		if err != nil {
			ctx.Logger().Error().
				Err(err).
				Str("action_description", a.Description).
				Msg("failed to take action")
		}
	}

	ctx.Logger().Debug().
		Str("text", me.Text).
		Int("actions", len(actions)).
		Msg("message handled")

	return false, false, nil
}

func onlyOtherUserMMentions(selfID string, mentions []mparser.Mention) ([]mparser.Mention, bool) {
	if len(mentions) == 0 {
		return nil, false
	}

	var self bool

	fm := make([]mparser.Mention, 0, len(mentions)-1)

	for _, m := range mentions {
		if m.Type != mparser.TypeUser {
			continue
		}

		if m.ID == selfID {
			self = true
			continue
		}

		fm = append(fm, m)
	}

	return fm, self
}

func isDM(c ChannelType) bool {
	switch c {
	case ChannelPublic, ChannelPrivate:
		return false
	case ChannelDM, ChannelGroupDM, ChannelAppHome:
		return true
	default:
		return false
	}
}

// Match looks at the trigger to see if it matches any known handlers. Some
// handlers are only invoked if the bot was mentioned.
func (m *MessageActions) Match(message Message) []MessageAction {
	message.text, message.allMentions = mparser.ParseAndSplice(message.rawText, message.channelID)
	message.text = strings.TrimSpace(message.text)

	message.userMentions, message.botMentioned = onlyOtherUserMMentions(m.selfID, message.allMentions)

	t := message.text
	lt := strings.ToLower(t) // for where we can't easily use EqualFold()

	// if this is an alias, do the swap
	if v, ok := m.aliases[lt]; ok {
		t = v
		lt = v
	}

	var aa []MessageAction

	dm := isDM(message.channelType)

	if dm || !m.shadowMode {
		for k, v := range m.reactions {
			if strings.Contains(lt, k) && (!v.onlyWhenMentioned || message.botMentioned) {
				a := MessageAction{
					Self:        k,
					Description: v.description,
					fn:          v.fn,
					m:           message,
				}
				aa = append(aa, a)
			}
		}

		for k, v := range m.prefixResponses {
			if strings.HasPrefix(lt, k) {
				a := MessageAction{
					Self:        k,
					Description: v.description,
					fn:          v.fn,
					m:           message,
				}
				aa = append(aa, a)
			}
		}
	}

	if dm || message.botMentioned {
		for k, v := range m.responses {
			if strings.EqualFold(k, t) {
				a := MessageAction{
					Self:        k,
					Description: v.description,
					fn:          v.fn,
					m:           message,
				}
				aa = append(aa, a)
			}
		}
	}

	for _, v := range m.dynamic {
		if v.matchfn(message) {
			a := MessageAction{
				Description: v.description,
				fn:          v.fn,
				m:           message,
			}

			aa = append(aa, a)
		}
	}

	return aa
}

// Handle is similar to HandleStatic(), but instead of specifying which content
// to reply with you provide your own MessageActionFn function. The bot must be
// mentioned for this to match. This also supports aliases so that you can have
// shorter versions.
func (m *MessageActions) Handle(trigger, description string, aliases []string, fn MessageActionFn) {
	if len(trigger) == 0 {
		panic("trigger cannot be empty string")
	}

	if fn == nil {
		panic("fn cannot be nil")
	}

	if _, ok := m.responses[trigger]; ok {
		panic(fmt.Sprintf("trigger %q already exists", trigger))
	}

	for _, a := range aliases {
		if _, ok := m.aliases[a]; ok {
			m.logger.Warn().
				Str("trigger", trigger).
				Str("alias", a).
				Msg("trigger alias already exists, skipping")
			continue
		}

		m.aliases[a] = trigger
	}

	m.responses[trigger] = reactiveAction{
		description: description,
		aliases:     aliases,
		fn:          fn,
	}
}

// HandleStatic is a static response handler. If the trigger matches a known
// value, return the content. The bot must be mentioned for this to match. It
// also supports aliases so that you can have shorter versions.
func (m *MessageActions) HandleStatic(trigger, description string, aliases []string, content ...string) {
	msg := strings.Join(content, "\n")

	fn := func(ctx workqueue.Context, m Messenger, r Responder) error {
		return r.RespondMentions(ctx, msg)
	}

	m.Handle(trigger, description, aliases, fn)
}

// HandleStaticContains handles reacting to messages that contain trigger
// anywhere in the message, except it responds instead of reacting with an
// emoji.
func (m *MessageActions) HandleStaticContains(contains string, content ...string) {
	if len(contains) == 0 {
		panic("contains cannot be empty string")
	}

	if len(content) == 0 {
		panic("reactions variadic cannot be empty")
	}

	if _, ok := m.responses[contains]; ok {
		panic(fmt.Sprintf("trigger %q already exists", contains))
	}

	msg := strings.Join(content, "\n")

	m.reactions[contains] = reactiveAction{
		fn: func(ctx workqueue.Context, m Messenger, r Responder) error {
			return r.Respond(ctx, msg)
		},
	}
}

// HandleReaction handles reacting to messages that contain trigger anywhere in
// the message.
func (m *MessageActions) HandleReaction(trigger string, reactions ...string) {
	if len(trigger) == 0 {
		panic("trigger cannot be empty string")
	}

	if len(reactions) == 0 {
		panic("reactions variadic cannot be empty")
	}

	if _, ok := m.responses[trigger]; ok {
		panic(fmt.Sprintf("trigger %q already exists", trigger))
	}

	m.reactions[trigger] = reactiveAction{
		fn: reactionFactory(false, 0, reactions...),
	}
}

// HandleMentionedReaction handles reacting to messages that contain trigger anywhere in
// the message, but only if the bot is mentioned.
func (m *MessageActions) HandleMentionedReaction(trigger string, reactions ...string) {
	if len(trigger) == 0 {
		panic("trigger cannot be empty string")
	}

	if len(reactions) == 0 {
		panic("reactions variadic cannot be empty")
	}

	if _, ok := m.responses[trigger]; ok {
		panic(fmt.Sprintf("trigger %q already exists", trigger))
	}

	m.reactions[trigger] = reactiveAction{
		onlyWhenMentioned: true,
		fn:                reactionFactory(false, 0, reactions...),
	}
}

// HandleReactionRand handles reacting to messages that contain trigger anywhere in
// the message, but only doing it periodically.
func (m *MessageActions) HandleReactionRand(trigger string, reactions ...string) {
	if len(trigger) == 0 {
		panic("trigger cannot be empty string")
	}

	if len(reactions) == 0 {
		panic("reactions variadic cannot be empty")
	}

	if _, ok := m.responses[trigger]; ok {
		panic(fmt.Sprintf("trigger %q already exists", trigger))
	}

	m.reactions[trigger] = reactiveAction{
		fn: reactionFactory(true, 0x2A, reactions...),
	}
}

func reactionFactory(random bool, randFactor int, reactions ...string) func(ctx workqueue.Context, m Messenger, r Responder) error {
	return func(ctx workqueue.Context, m Messenger, r Responder) error {
		if random && rand.Intn(150) != 0x2A { // not this time, maybe next time!
			return nil
		}

		for _, reaction := range reactions {
			if err := r.React(ctx, reaction); err != nil {
				// if err := ctx.Slack().AddReactionContext(ctx, r, slack.ItemRef{Channel: channelID, Timestamp: messageTS}); err != nil {
				return fmt.Errorf("failed to react with %s: %w", r, err)
			}
		}

		return nil
	}
}

// HandlePrefix handles a message with any prefix, regardless of the mentions in
// it.
func (m *MessageActions) HandlePrefix(prefix, description string, fn MessageActionFn) {
	if len(prefix) == 0 {
		panic("prefix cannot be empty string")
	}

	if fn == nil {
		panic("fn cannot be nil")
	}

	if _, ok := m.prefixResponses[prefix]; ok {
		panic(fmt.Sprintf("prefix %q already exists", prefix))
	}

	m.prefixResponses[prefix] = reactiveAction{
		description: description,
		fn:          fn,
	}
}

// HandleDynamic allows you to define a handler where you control whether it
// matches by providing your own MessageMatchFn. This allows for the handler to
// be dynamic.
func (m *MessageActions) HandleDynamic(matchFn MessageMatchFn, actionFn MessageActionFn) {
	ra := reactiveAction{
		fn:      actionFn,
		matchfn: matchFn,
	}

	m.dynamic = append(m.dynamic, ra)
}
