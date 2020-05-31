package main

import (
	"fmt"

	"github.com/gobridge/gopherbot/handler"
	"github.com/gobridge/gopherbot/workqueue"
)

func injectChannelJoinHandlers(c *handler.ChannelJoinActions) {
	c.Handle("newbie", "C02A8LZKT",
		func(ctx workqueue.Context, cj handler.ChannelJoiner, r handler.Responder) error {
			msg := newbiesWelcomeMessage(ctx.Self().ID)

			ctx.Logger().Debug().
				Str("channel_id", cj.ChannelID()).
				Str("user_id", cj.UserID()).
				Time("joined_time", ctx.Meta().Time).
				Int("msg_len", len(msg)).
				Msg("welcoming user to newbies channel")

			return r.RespondEphemeral(ctx, msg)
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
