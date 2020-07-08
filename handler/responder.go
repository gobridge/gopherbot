package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/gobridge/gopherbot/mparser"
	"github.com/slack-go/slack"
)

// Responder is the interface to describe the functionality used by handlers to
// respond or react.
type Responder interface {
	React(ctx context.Context, emoji string) error

	Respond(ctx context.Context, msg string, attachments ...slack.Attachment) error

	// RespondTo is the same as respond, except it prefixes the message with an
	// at-mention of the user who triggered the action. Helpful if responding
	// with an error message.
	RespondTo(ctx context.Context, msg string, attachments ...slack.Attachment) error

	// RespondUnfurled is the same as Respond, except it asks slack to redner
	// URL previews in the channel or DM.
	RespondUnfurled(ctx context.Context, msg string, attachments ...slack.Attachment) error

	// RespondTextAttachment responds in the channel or thread with a text
	// attachment (helpful for sharing long messages).
	RespondTextAttachment(ctx context.Context, msg, attachment string) error

	// RespondMentions responds in the channel / thread, and mentions any users
	// who were mentioned in the original message.
	RespondMentions(ctx context.Context, msg string, attachments ...slack.Attachment) error

	// RespondMentionsUnfurled is the same as RespondMentions, but with
	// Unfurling enabled like RespondUnfurled.
	RespondMentionsUnfurled(ctx context.Context, msg string, attachments ...slack.Attachment) error

	// RespondMentionsTextAttachment is similar to RespondMentions, except with
	// the additional text attachment.
	RespondMentionsTextAttachment(ctx context.Context, msg, attachment string) error

	// RespondEphemeral responds with a message only the person who sent the message will see.
	RespondEphemeral(ctx context.Context, msg string, attachments ...slack.Attachment) error

	// RespondEphemeralTextAttachment is similar to RespondEphemeral, but also
	// includes a text attachment.
	RespondEphemeralTextAttachment(ctx context.Context, msg, attachment string) error

	// RespondeDM is for sending a DM to the user instead of responding in
	// the channel, or with an ephemeral message.
	RespondDM(ctx context.Context, msg string, attachments ...slack.Attachment) error
}

type response struct {
	sc *slack.Client
	m  Message
}

// interface implementation check
var _ Responder = response{}

func (r response) React(ctx context.Context, emoji string) error {
	item := slack.ItemRef{
		Channel:   r.m.channelID,
		Timestamp: r.m.messageTS,
	}

	if err := r.sc.AddReactionContext(ctx, emoji, item); err != nil {
		return fmt.Errorf("failed to AddReactionContext: %w", err)
	}

	return nil
}

func (r response) Respond(ctx context.Context, msg string, attachments ...slack.Attachment) error {
	return r.respond(ctx, false, false, false, false, r.m.channelID, r.m.threadTS, r.m.subType, msg, attachments...)
}

func (r response) RespondTo(ctx context.Context, msg string, attachments ...slack.Attachment) error {
	return r.respond(ctx, true, false, false, false, r.m.channelID, r.m.threadTS, r.m.subType, msg, attachments...)
}

func (r response) RespondDM(ctx context.Context, msg string, attachments ...slack.Attachment) error {
	return r.respond(ctx, false, false, false, false, r.m.userID, r.m.threadTS, r.m.subType, msg, attachments...)
}

func (r response) RespondUnfurled(ctx context.Context, msg string, attachments ...slack.Attachment) error {
	return r.respond(ctx, false, false, false, true, r.m.channelID, r.m.threadTS, r.m.subType, msg, attachments...)
}

func (r response) RespondTextAttachment(ctx context.Context, msg, attachment string) error {
	return r.respond(ctx, false, false, false, false, r.m.channelID, r.m.threadTS, r.m.subType, msg, slack.Attachment{Text: attachment})
}

func (r response) RespondMentions(ctx context.Context, msg string, attachments ...slack.Attachment) error {
	return r.respond(ctx, false, true, false, false, r.m.channelID, r.m.threadTS, r.m.subType, msg, attachments...)
}

func (r response) RespondMentionsUnfurled(ctx context.Context, msg string, attachments ...slack.Attachment) error {
	return r.respond(ctx, false, true, false, true, r.m.channelID, r.m.threadTS, r.m.subType, msg, attachments...)
}

func (r response) RespondMentionsTextAttachment(ctx context.Context, msg, attachment string) error {
	return r.respond(ctx, false, true, false, false, r.m.channelID, r.m.threadTS, r.m.subType, msg, slack.Attachment{Text: attachment})
}

func (r response) RespondEphemeral(ctx context.Context, msg string, attachments ...slack.Attachment) error {
	return r.respond(ctx, true, false, true, false, r.m.channelID, r.m.threadTS, r.m.subType, msg, attachments...)
}

func (r response) RespondEphemeralTextAttachment(ctx context.Context, msg, attachment string) error {
	return r.respond(ctx, true, false, true, false, r.m.channelID, r.m.threadTS, r.m.subType, msg, slack.Attachment{Text: attachment})
}

func (r response) respond(ctx context.Context, mentionUser, useMentions, ephemeral, unfurled bool, channelID, threadTS, subType, msg string, attachments ...slack.Attachment) error {
	if useMentions && ephemeral {
		return errors.New("cannot use mentions for ephemeral messages")
	}

	if useMentions && len(r.m.userMentions) > 0 {
		msg = mparser.Join(r.m.userMentions, " ") + msg
	}

	// do this after the above, so the original user is first in the message
	if mentionUser {
		u := mparser.Mention{
			ID:   r.m.userID,
			Type: mparser.TypeUser,
		}

		msg = fmt.Sprintf("%s %s", u.String(), msg)
	}

	var opts []slack.MsgOption

	if unfurled {
		opts = append(opts, slack.MsgOptionEnableLinkUnfurl())
	} else {
		opts = append(opts,
			slack.MsgOptionDisableLinkUnfurl(),
			slack.MsgOptionDisableMediaUnfurl(),
		)
	}

	opts = append(opts, slack.MsgOptionText(msg, false))

	if len(threadTS) > 0 {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	// if it's a command that was triggered in a shared thread reply
	// we should share our reply with the channel too
	if len(subType) > 0 && subType == "thread_broadcast" {
		opts = append(opts, slack.MsgOptionBroadcast())
	}

	if len(attachments) > 0 {
		opts = append(opts, slack.MsgOptionAttachments(attachments...))
	}

	if ephemeral {
		if _, err := r.sc.PostEphemeralContext(ctx, channelID, r.m.userID, opts...); err != nil {
			return fmt.Errorf("failed to PostEphemeralContext: %w", err)
		}
	} else {
		if _, _, _, err := r.sc.SendMessageContext(ctx, channelID, opts...); err != nil {
			return fmt.Errorf("failed to SendMessageContext: %w", err)
		}
	}

	return nil
}
