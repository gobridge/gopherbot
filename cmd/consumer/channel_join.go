package main

import (
	"context"
	"fmt"

	"github.com/gobridge/gopherbot/handler"
	"github.com/gobridge/gopherbot/workqueue"
	"github.com/slack-go/slack"
)

func injectChannelJoinHandlers(c *handler.ChannelJoinActions) {
	c.Handle("newbie", "C02A8LZKT",
		func(ctx workqueue.Context, cj handler.ChannelJoiner, r handler.Responder) error {
			msg := newbiesWelcomeMessage(ctx.Self().ID)
			msg = "Would weclome <@" + cj.UserID() + "> to newbies channel\n\n" + msg

			ctx.Logger().Debug().
				Str("channel_id", cj.ChannelID()).
				Str("user_id", cj.UserID()).
				Time("joined_time", ctx.Meta().Time).
				Int("msg_len", len(msg)).
				Msg("welcoming user to newbies")

			opts := []slack.MsgOption{
				slack.MsgOptionDisableLinkUnfurl(),
				slack.MsgOptionDisableMediaUnfurl(),
				slack.MsgOptionText(msg, false),
			}

			// sending to #gopherdev
			_, _, _, err := ctx.Slack().SendMessageContext(context.TODO(), "C013XC5SU21", opts...)
			return err
		},
	)
}

const newbiesWelcomeMessageFormat = `welcome to <#C02A8LZKT>: the channel for newbies to Go, or programming in general, to learn together.

Please consider introducing yourself in the channel, maybe sharing where you're from, your programming background, and how you'd like to use Go.

I am the community chat bot and have some resources available for you to get started. If you'd like to see them, please type: <@%s> newbie resources

You can also ask me for all the commands I support: <@%s> help

We hope you have fun learning Go! :gopherdance:`

func newbiesWelcomeMessage(selfID string) string {
	return fmt.Sprintf(newbiesWelcomeMessageFormat, selfID, selfID)
}

/*
// TODO(theckman): figure out a better way to handle these
const (
	bkennedyID  = "U029RQSE8"
	sausheongID = "U03QZHXD8"
)


func welcomeMessage(channels []recommendedChannel, cs workqueue.ChannelSvc) string {
	b := &strings.Builder{}

	var generalID string

	for _, c := range channels {
		if c.welcome {
			if ch, ok := cs.Lookup(c.name); ok {
				if c.name == "general" {
					generalID = ch.ID
				}
				fmt.Fprintf(b, "- <#%s> -> %s\n", ch.ID, c.desc)
			}
		}
	}

	return fmt.Sprintf(welcomeMessageFormat, b.String(), bkennedyID, sausheongID, generalID)
}

const welcomeMessageFormat = `Welcome to the Gophers Slack channel.
This Slack is meant to connect gophers from all over the world in a central place.
There is also a forum: <https://forum.golangbridge.org>, you might want to check it out as well.
We have a few rules that you can see here: <http://coc.golangbridge.org>.

Here's a list of a few channels you could join:
%s

If you want more suggestions, type "recommended channels".
There are quite a few other channels, depending on your interests or location (we have city / country wide channels).
Just click on the channel list and search for anything that crosses your mind.

To share code, you should use: <https://play.golang.org/> as it makes it easy for others to help you.

If you are new to Go and want a copy of the Go In Action book, <https://www.manning.com/books/go-in-action>, please send an email to <@%s> at bill@ardanlabs.com

If you are interested in a free copy of the Go Web Programming book by Sau Sheong Chang, <@%s>, please send him an email at sausheong@gmail.com

In case you want to customize your profile picture, you can use <https://gopherize.me/> to create a custom gopher.

Final thing, <#%s> might be too chatty at times but don't be shy to ask your Go related question.


Now, enjoy the community and have fun.`
*/
