package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gobridge/gopherbot/handler"
	"github.com/gobridge/gopherbot/workqueue"
	"github.com/slack-go/slack"
)

func injectTeamJoinHandlers(t *handler.TeamJoinActions) {
	t.Handle("new members",
		func(ctx workqueue.Context, tj handler.TeamJoiner, r handler.Responder) error {
			wmsg, err := welcomeMessage(recommendedChannels, ctx.ChannelSvc(), ctx.Self().ID, ctx.Self().Name)
			if err != nil {
				return fmt.Errorf("failed to generate welcome message: %w", err)
			}

			msg := "Would weclome <@" + tj.User().ID + ">:\n\n" + wmsg

			ctx.Logger().Debug().
				Str("user_id", tj.User().ID).
				Str("user_email", tj.User().Profile.Email).
				Time("joined_time", ctx.Meta().Time).
				Int("msg_len", len(msg)).
				Msg("welcoming user")

			opts := []slack.MsgOption{
				slack.MsgOptionDisableLinkUnfurl(),
				slack.MsgOptionDisableMediaUnfurl(),
				slack.MsgOptionText(msg, false),
			}

			_, _, _, err = ctx.Slack().SendMessageContext(context.TODO(), "C013XC5SU21", opts...)
			return err

			// return r.RespondPrivate(ctx, wm)
		},
	)
}

// TODO(theckman): figure out a better way to handle these
const (
	bkennedyID  = "U029RQSE8"
	sausheongID = "U03QZHXD8"
)

func welcomeMessage(channels []recommendedChannel, cs workqueue.ChannelSvc, selfID, selfName string) (string, error) {
	b := &strings.Builder{}

	var generalID, adminHelpID string

	for _, c := range channels {
		if c.welcome {
			ch, notFound, err := cs.Lookup(c.name)
			if err != nil {
				return "", fmt.Errorf("failed to look up channel: %w", err)
			}

			if notFound {
				continue // weird...
			}

			switch c.name {
			case "general":
				generalID = ch.ID
			case "admin-help":
				adminHelpID = ch.ID
			}

			fmt.Fprintf(b, "- <#%s> -> %s\n", ch.ID, c.desc)

		}
	}

	return fmt.Sprintf(teamJoinWelcomeMessageFormat, adminHelpID, selfID, selfName, generalID, b.String(), bkennedyID, sausheongID), nil
}

// because of the usage of backticks and quotes in the welcome message, this
// constant has become a bit ridiculous.
//
// maybe it would be easier to read if it were a slice of strings?
const teamJoinWelcomeMessageFormat = `Welcome to the Gophers Slack Workspace! This space is meant to connect gophers from all over the world in a central place. I am the community chat bot, and do have a few functions available to help you during your time here. :simple_smile:

Before getting started, we ask that you take a look at the rules all members are expected to follow: <http://coc.golangbridge.org>. If you ever need help from our workspace's community moderators or administrators, please reach out in <#%s>.

If you'd like to learn more about the functions I offer, please send me the ` + " `help` " + `command. You can send commands to me via a DM (like this one), or by mentioning me (<@%s>) in one of the main public channels:

` + "```" + `
@%s help
` + "```" + `

There is also a forum <https://forum.golangbridge.org>, which you might want to check it out as well if a Forum is more your style.

<#%s> can sometimes seem busy sometimes, but please don't hesitate to ask your Go related questions there. To share code while asking a question, you should use: <https://play.golang.org/> as it makes it easy for others to help you.

Here's a list of a few other channels you could join:
%s

If you want more channel suggestions, type` + " `recommended channels` " + `in a direct message to me.

There are quite a few other channels, depending on your interests or location (we have city / country wide channels). Just click on the :heavy_plus_sign: next to the channel list in the sidebar, and click Browse Channels to search for anything that interests you.

If you are new to Go and want a copy of the Go In Action book, <https://www.manning.com/books/go-in-action>, please send an email to <@%s> at bill@ardanlabs.com

If you are interested in a free copy of the Go Web Programming book by Sau Sheong Chang, <@%s>, please send him an email at sausheong@gmail.com

In case you want to customize your profile picture, you can use <https://gopherize.me/> to create a custom gopher.

Now, enjoy the community and have fun! :gopher:`
