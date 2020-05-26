package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gobridge/gopherbot/handler"
	"github.com/gobridge/gopherbot/workqueue"
)

var xkcdAliases = map[string]uint64{
	"standards":    927,
	"compiling":    303,
	"optimization": 1691,
}

func injectMessageResponsePrefix(ma *handler.MessageActions) {
	ma.HandlePrefix("xkcd:", "helpfully give you the XKCD link you want",
		func(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
			parts := strings.Split(m.Text(), ":")

			if len(parts) != 2 || len(parts[1]) == 0 {
				return r.RespondMentions(ctx, "That was almost right. Proper format is `xkcd:1234`")
			}

			i := strings.IndexAny(parts[1], " \n")
			if i == -1 {
				i = len(parts[1])
			}

			idStr := parts[1][:i]

			comicID, ok := xkcdAliases[idStr]
			if !ok {
				u64, err := strconv.ParseUint(idStr, 10, 64)
				if err != nil {
					return r.RespondMentions(ctx, "That was almost right. Proper format is `xkcd:1234`")
				}

				comicID = u64
			}

			return r.RespondMentionsUnfurled(ctx, fmt.Sprintf("https://xkcd.com/%d", comicID))
		},
	)

	ma.HandlePrefix("d/", "generate a GoDoc.org link", godocLinkFactory("https://godoc.org/"))
	ma.HandlePrefix("ghd/", "generate a GoDoc.org link", godocLinkFactory("https://godoc.org/github.com/"))
}

func godocLinkFactory(prefix string) func(workqueue.Context, handler.Messenger, handler.Responder) error {
	return func(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
		text := strings.TrimPrefix(m.Text(), prefix)

		i := strings.IndexByte(text, '/')
		link := text[i+1:]

		if i := strings.Index(link, " "); i > -1 {
			link = link[:i]
		}

		if len(link) == 0 {
			return nil
		}

		return r.Respond(ctx, `<`+prefix+link+`>`)
	}
}
