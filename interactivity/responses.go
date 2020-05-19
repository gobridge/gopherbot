package interactivity

import (
	"errors"
	"math/rand"

	"github.com/slack-go/slack"
	"github.com/theckman/gopher2/mparser"
	"github.com/theckman/gopher2/workqueue"
)

func injectMessageResponseFuncs(r *MessageActions) {
	r.HandleFunc("flip a coin", "flips a coin, returning heads or tails",
		func(ctx workqueue.Context, channelID, messageTS string, mentions []mparser.Mention, _ string) error {
			var m string
			if rand.Intn(2) == 0 {
				m = "heads"
			} else {
				m = "tails"
			}

			_, _, _, err := ctx.Slack().SendMessageContext(ctx, channelID, slack.MsgOptionText(m, true))
			return err
		},
	)

	r.HandleFunc("help", "show help",
		func(ctx workqueue.Context, channelID, messageTS string, mentions []mparser.Mention, _ string) error {
			return errors.New("implement me")
		},
	)
}

func injectMessageResponses(r *MessageActions) {
	r.Handle("recommended blogs", "returns a list of recommended blogs or twitter feeds",
		`Here are some popular blog posts and Twitter accounts you should follow:`,
		`- Peter Bourgon <https://twitter.com/peterbourgon|@peterbourgon> - <https://peter.bourgon.org/blog>`,
		`- Carlisia Campos <https://twitter.com/carlisia|@carlisia>`,
		`- Dave Cheney <https://twitter.com/davecheney|@davecheney> - <http://dave.cheney.net>`,
		`- Jaana Burcu Dogan <https://twitter.com/rakyll|@rakyll> - <http://golang.rakyll.org>`,
		`- Jessie Frazelle <https://twitter.com/jessfraz|@jessfraz> - <https://blog.jessfraz.com>`,
		`- William "Bill" Kennedy <https://twitter.com|@goinggodotnet> - <https://www.goinggo.net>`,
		`- Brian Ketelsen <https://twitter.com/bketelsen|@bketelsen> - <https://www.brianketelsen.com/blog>`,
	)

	r.Handle("books", "returns a list of books about Go that can help you get started",
		`Here are some popular books you can use to get started:`,
		`- William Kennedy, Brian Ketelsen, Erik St. Martin Go In Ajction <https://www.manning.com/books/go-in-action>`,
		`- Alan A A Donovan, Brian W Kernighan The Go Programming Language <https://www.gopl.io>`,
		`- Mat Ryer Go Programming Blueprints 2nd Edition <https://www.packtpub.com/application-development/go-programming-blueprints-second-edition>`,
	)

	r.Handle("oss help wanted", "find projects with help wanted",
		`Here's a list of projects which could need some help from contributors like you: <https://github.com/corylanou/oss-helpwanted>`,
	)

	r.Handle("working with forks", "info on how to work with forks in Go",
		`Here's how to work with package forks in Go: <http://blog.sgmansfield.com/2016/06/working-with-forks-in-go/>`,
	)

	r.Handle("block forever", "how to block forever",
		`Here's how to block forever in Go: <http://blog.sgmansfield.com/2016/06/how-to-block-forever-in-go/>`,
	)

	r.Handle("http timeouts", "info on http timeouts in Go",
		`Here's a blog post which will help with http timeouts in Go: <https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/>`,
	)

	r.Handle("slices", "info on slices and how to use them",
		`The following posts will explain how slices, maps and strings work in Go:`,
		`- <https://blog.golang.org/go-slices-usage-and-internals>`,
		`- <https://blog.golang.org/slices>`,
		`- <https://blog.golang.org/strings>`,
	)

	r.Handle("database tutorial", "working with SQL in Go",
		`Here's how to work with database/sql in Go: <http://go-database-sql.org/>`,
	)

	r.Handle("package layout", "guidance on how to structure your Go projects",
		`These articles will explain how to organize your Go packages:`,
		`- <https://rakyll.org/style-packages/>`,
		`- <https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1#.ds38va3pp>`,
		`- <https://peter.bourgon.org/go-best-practices-2016/#repository-structure>`,
		``,
		`This article will help you understand the design philosophy for packages: <https://www.goinggo.net/2017/02/design-philosophy-on-packaging.html>`,
	)

	r.Handle("idiomatic go", "want tips on writing idiomatic Go?",
		`Tips on how to write idiomatic Go code <https://dmitri.shuralyov.com/idiomatic-go>`,
	)

	r.Handle("avoid gotchas", "avoid common Go gotchas",
		`Read this article if you want to understand and avoid common gotchas in Go <https://divan.github.io/posts/avoid_gotchas>`,
	)

	r.Handle("source code", "where does this bot's source live?",
		`My source code is here <https://github.com/gobridge/gopher>`,
	)

	r.Handle("dependency injection", "learn more about DI in Go",
		`If you'd like to learn more about how to use Dependency Injection in Go, please review this post:`,
		`- <https://appliedgo.net/di/>`,
	)

	r.Handle("pointer performance", "learn more about pointers and their impact on performance",
		`The answer to whether using a pointer offers a performance gain is complex and is not always the case. Please read these posts for more information:`,
		`- <https://medium.com/@vCabbage/go-are-pointers-a-performance-optimization-a95840d3ef85>`,
		`- <https://segment.com/blog/allocation-efficiency-in-high-performance-go-services/>`,
	)

	r.Handle("gopath", "learn more about using the GOPATH",
		"Your project should be structured as follows:",
		"```GOPATH=~/go",
		"~/go/src/sourcecontrol/username/project/```",
		"Whilst you _can_ get around the GOPATH, it's ill-advised. Read more about the GOPATH here: https://github.com/golang/go/wiki/GOPATH",
	)
}
