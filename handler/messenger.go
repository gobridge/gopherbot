package handler

import (
	"github.com/slack-go/slack/slackevents"
	"github.com/gobridge/gopherbot/mparser"
)

// ChannelType represents where a message was sent.
type ChannelType uint8

const (
	// ChannelUnknown is for when the type wasn't provided.
	ChannelUnknown ChannelType = iota

	// ChannelPublic is for public channels.
	ChannelPublic ChannelType = 1<<iota - 1

	// ChannelPrivate is for private channels, called groups by Slack API
	ChannelPrivate

	// ChannelDM is for direct messages, or called IMs by Slack API
	ChannelDM

	// ChannelGroupDM is for group direct messages, or called MPIMs by Slack API
	ChannelGroupDM

	// ChannelAppHome is for messages sent to the AppHome.
	ChannelAppHome
)

func (c ChannelType) String() string {
	switch c {
	case ChannelPublic:
		return "public"
	case ChannelPrivate:
		return "private"
	case ChannelDM:
		return "dm"
	case ChannelGroupDM:
		return "group_dm"
	case ChannelAppHome:
		return "app_home"
	default:
		return "unknown"
	}
}

func strToChan(s string) ChannelType {
	switch s {
	case "channel":
		return ChannelPublic
	case "group":
		return ChannelPrivate
	case "im":
		return ChannelDM
	case "mpim":
		return ChannelGroupDM
	case "app_home":
		return ChannelAppHome
	default:
		return ChannelUnknown
	}
}

// Messenger is the interface to represent an incoming message.
type Messenger interface {
	// ChannelID is the ID of the channel where the message was sent. This could
	// be a DM and not a channel, so be sure to check ChannelType()
	ChannelID() string

	// ChannelType is the kind of channel this is, also includes DMs.
	ChannelType() ChannelType

	// UserID is the ID of the user who sent the message.
	UserID() string

	// ThreadTS is the parent message ID, indicating we are in a thread
	ThreadTS() string

	// MessageTS is the message ID. If ThreadTS is empty string, we're not in a
	// thread
	MessageTS() string

	// AllMentions contains all parsed mentions in a message, including the bot
	// user, channels, etc.
	AllMentions() []mparser.Mention

	// UserMentions() contains only non-bot user mentions in the message.
	UserMentions() []mparser.Mention

	// Text is the text with any mentions removed, and leading/trailing
	// whitespace removed.
	Text() string

	// RawText is the raw Slack messages with no mentions removed.
	RawText() string

	// BotMentioned indicates if the bot was mentioned in the message.
	BotMentioned() bool

	// Files are any files attached to the message
	Files() []slackevents.File
}

// Message is a singular message to be processed. Satisfies Messenger interface.
type Message struct {
	channelID    string
	channelType  ChannelType
	userID       string
	threadTS     string
	messageTS    string
	allMentions  []mparser.Mention
	userMentions []mparser.Mention
	text         string
	botMentioned bool
	rawText      string
	files        []slackevents.File
}

var _ Messenger = Message{}

// NewMessage generates a new message from the various inputs.
func NewMessage(channelID, channelType, userID, threadTS, messageTS, text string, files []slackevents.File) Message {
	return Message{
		channelID:   channelID,
		channelType: strToChan(channelType),
		userID:      userID,
		threadTS:    threadTS,
		messageTS:   messageTS,
		rawText:     text,
		files:       files,
	}
}

// ChannelID satisfies the Messenger interface.
func (m Message) ChannelID() string { return m.channelID }

// ChannelType satisfies the Messenger interface.
func (m Message) ChannelType() ChannelType { return m.channelType }

// UserID satisfies the Messenger interface.
func (m Message) UserID() string { return m.userID }

// ThreadTS satisfies the Messenger interface.
func (m Message) ThreadTS() string { return m.threadTS }

// MessageTS satisfies the Messenger interface.
func (m Message) MessageTS() string { return m.messageTS }

// AllMentions satisfies the Messenger interface.
func (m Message) AllMentions() []mparser.Mention { return m.allMentions }

// UserMentions satisfies the Messenger interface.
func (m Message) UserMentions() []mparser.Mention { return m.userMentions }

// Text satisfies the Messenger interface.
func (m Message) Text() string { return m.text }

// RawText satisfies the Messenger interface.
func (m Message) RawText() string { return m.rawText }

// BotMentioned satisfies the Messenger interface.
func (m Message) BotMentioned() bool { return m.botMentioned }

// Files satisfies the Messenger interface.
func (m Message) Files() []slackevents.File { return m.files }
