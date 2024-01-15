package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/gobridge/gopherbot/handler"
	"github.com/gobridge/gopherbot/mparser"
	"github.com/gobridge/gopherbot/workqueue"
)

func fmtAliases(s []string) []string {
	ss := make([]string, len(s))
	for i := range s {
		ss[i] = fmt.Sprintf("`%s`", s[i])
	}
	return ss
}

type recommendedChannel struct {
	name    string
	desc    string
	welcome bool
}

var recommendedChannels = []recommendedChannel{
	{name: "general", desc: "for general Go questions or help", welcome: true},
	{name: "newbies", desc: "for newbie resources", welcome: true},
	{name: "gotimefm", desc: "for the awesome live podcast", welcome: true},
	{name: "jobs", desc: "for jobs related to Go", welcome: true},
	{name: "modules", desc: "for help with Go Modules"},
	{name: "reviews", desc: "for code reviews", welcome: true},
	{name: "showandtell", desc: "for telling the world about the thing you are working on"},
	{name: "aws", desc: "if you are interested in AWS"},
	{name: "bbq", desc: "Go controlling your bbq grill? Yes, we have that"},
	{name: "devops", desc: "for DevOps related discussions"},
	{name: "golang-cls", desc: "get real time updates from merged CL for Go itself"},
	{name: "goreviews", desc: "talk to the Go team about a certain CL", welcome: true},
	{name: "performance", desc: "anything and everything performance related"},
	{name: "remotemeetup", desc: "for remote meetups", welcome: true},
	{name: "security", desc: "for security related discussions"},
	{name: "admin-help", desc: "for engaging with the moderators / admins of this Slack workspace", welcome: true},
}

const newbiesChanID = "C02A8LZKT"

func injectMessageResponseFuncs(ma *handler.MessageActions) {
	ma.Handle("flip a coin", "flips a coin, returning heads or tails", []string{"flip coin", "coin flip"},
		func(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
			var msg string
			if rand.Intn(2) == 0 {
				msg = "heads"
			} else {
				msg = "tails"
			}

			err := r.Respond(ctx, msg)

			return err
		},
	)

	ma.Handle("newbie resources", "resources for newbies", nil,
		func(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
			msg := "Here are some resources you should check out if you are learning / new to Go:"

			if m.ChannelID() != newbiesChanID {
				cmnt := mparser.Mention{
					Type: mparser.TypeChannelRef,
					ID:   newbiesChanID,
				}.String()

				msg = fmt.Sprintf("If you'd like to join others who are new to Go, we have the %s channel.\n\n%s", cmnt, msg)
			}

			return r.RespondMentionsTextAttachment(
				ctx,
				msg,
				newbieResourcesMessage,
			)
		},
	)

	ma.Handle("recommended channels", "channels we recommend folks join", []string{"channels"},
		func(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
			builder := &strings.Builder{}

			for _, channel := range recommendedChannels {
				c, notFound, err := ctx.ChannelSvc().Lookup(channel.name)
				if err != nil {
					return fmt.Errorf("failed to look up channel: %w", err)
				}

				if notFound {
					continue // weird...
				}

				fmt.Fprintf(builder, "- <#%s> -> %s\n", c.ID, channel.desc)

			}

			return r.RespondMentionsTextAttachment(ctx, "Here is a list of recommended channels", builder.String())
		},
	)

	ma.Handle("help", "show the commands I support", []string{"commands"},
		func(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
			hs := ma.Registered()
			sort.Slice(hs, func(i, j int) bool {
				if hs[i].Trigger == hs[j].Trigger {
					return hs[i].Description < hs[j].Description
				}

				return hs[i].Trigger < hs[j].Trigger
			})

			b := &strings.Builder{}

			var hasPrefix bool

			for _, h := range hs {
				if h.Prefix {
					hasPrefix = true
					continue
				}

				// print each command, with aliases on their own line
				fmt.Fprintf(b, "- `%s`: %s\n", h.Trigger, h.Description)

				if len(h.Aliases) > 0 {
					a := strings.Join(fmtAliases(h.Aliases), ",")
					fmt.Fprintf(b, "\t- aliases: %s\n", a)
				}

				fmt.Fprintln(b)
			}

			// if we have some prefixed commands, do it again
			if hasPrefix {
				fmt.Fprint(b, "\n\nThere are also these special message prefixes:\n\n")

				for _, h := range hs {
					if !h.Prefix {
						continue
					}

					fmt.Fprintf(b, "- `%s`: %s\n\n", h.Trigger, h.Description)
				}
			}

			return r.RespondMentionsTextAttachment(ctx, "I respond to the following commands in public channels, or via a direct (private) message:", b.String())
		},
	)
}

func injectMessageResponses(ma *handler.MessageActions) {
	ma.HandleStatic("recommended", "returns a list of recommended blogs or twitter feeds", []string{"recommended blogs"},
		`Here are some popular blog posts and Twitter accounts you should follow:`,
		`- Peter Bourgon <https://twitter.com/peterbourgon|@peterbourgon> - <https://peter.bourgon.org/blog>`,
		`- Carlisia Campos <https://twitter.com/carlisia|@carlisia>`,
		`- Dave Cheney <https://twitter.com/davecheney|@davecheney> - <http://dave.cheney.net>`,
		`- Jaana Burcu Dogan <https://twitter.com/rakyll|@rakyll> - <http://golang.rakyll.org>`,
		`- Jessie Frazelle <https://twitter.com/jessfraz|@jessfraz> - <https://blog.jessfraz.com>`,
		`- William "Bill" Kennedy <https://twitter.com/goinggodotnet|@goinggodotnet> - <https://www.goinggo.net>`,
		`- Brian Ketelsen <https://twitter.com/bketelsen|@bketelsen> - <https://www.brianketelsen.com/blog>`,
	)

	ma.HandleStatic("books", "returns a list of books about Go that can help you get started", nil,
		`Here are some popular books you can use to get started:`,
		`- William Kennedy, Brian Ketelsen, Erik St. Martin Go In Action <https://www.manning.com/books/go-in-action>`,
		`- Alan A A Donovan, Brian W Kernighan The Go Programming Language <https://www.gopl.io>`,
		`- Mat Ryer Go Programming Blueprints 2nd Edition <https://www.packtpub.com/application-development/go-programming-blueprints-second-edition>`,
	)

	ma.HandleStatic("oss help wanted", "find projects with help wanted", []string{"help wanted", "oss help"},
		`Here's a list of projects which could need some help from contributors like you: <https://github.com/corylanou/oss-helpwanted>`,
	)

	ma.HandleStatic("work with forks", "info on how to work with forks in Go", []string{"working with forks"},
		`Here's how to work with package forks in Go: <http://blog.sgmansfield.com/2016/06/working-with-forks-in-go/>`,
	)

	ma.HandleStatic("block forever", "how to block forever", []string{"how to block forever"},
		"Here's the most common way to block forever in Go: `select {}`.",
		"For other ways check out this post: <https://blog.sgmansfield.com/2016/06/how-to-block-forever-in-go/>",
	)

	ma.HandleStatic("http timeouts", "info on http timeouts in Go", nil,
		`Here's a blog post which will help with http timeouts in Go: <https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/>`,
	)

	ma.HandleStatic("slices", "info on slices and how to use them", []string{"slice internals"},
		`The following posts will explain how slices, maps and strings work in Go:`,
		`- <https://blog.golang.org/go-slices-usage-and-internals>`,
		`- <https://blog.golang.org/slices>`,
		`- <https://blog.golang.org/strings>`,
	)

	ma.HandleStatic("database tutorial", "working with SQL in Go", []string{"databases"},
		`Here's how to work with database/sql in Go: <http://go-database-sql.org/>`,
	)

	ma.HandleStatic("empty struct", "information about values of type `struct{}`", []string{"zero struct", "struct{}"},
		"You can find out more about the `struct{}` type in this post by Dave Cheney: <https://dave.cheney.net/2014/03/25/the-empty-struct>",
	)

	ma.HandleStatic("project layout", "guidance on how to structure your Go projects", []string{"project structure", "package layout", "package structure"},
		`These articles will explain how to organize your Go packages:`,
		`- <https://rakyll.org/style-packages/>`,
		`- <https://www.gobeyond.dev/standard-package-layout/>`,
		`- <https://peter.bourgon.org/go-best-practices-2016/#repository-structure>`,
		`- <https://christine.website/blog/within-go-repo-layout-2020-09-07>`,
		``,
		`This article will help you understand the design philosophy for packages: <https://www.goinggo.net/2017/02/design-philosophy-on-packaging.html>`,
	)

	ma.HandleStatic("go challenges", "want to challenge your Go skills in interactive?", []string{"challenges", "challenge"},
		"Check out the growing list of Go challenges - <https://tutorialedge.net/challenges/go/>",
	)

	ma.HandleStatic("idiomatic go", "want tips on writing idiomatic Go?", nil,
		`Tips on how to write idiomatic Go code <https://dmitri.shuralyov.com/idiomatic-go>`,
	)

	ma.HandleStatic("invite link", "URL for getting an invite to the workspace", []string{"invite", "slack invite", "workspace invite"},
		`<https://invite.slack.golangbridge.org/>`,
	)

	ma.HandleStatic("avoid gotchas", "avoid common Go gotchas", []string{"gotchas"},
		`Read this article if you want to understand and avoid common gotchas in Go <https://divan.github.io/posts/avoid_gotchas>`,
	)

	ma.HandleStatic("source code", "where does this bot's source live?", []string{"source"},
		`You can find my source code, including all of my configured responses, here:`,
		`- <https://github.com/gobridge/gopherbot>`,
	)

	ma.HandleStatic("dependency injection", "learn more about DI in Go", []string{"di"},
		`If you'd like to learn more about how to use Dependency Injection in Go, please review this post:`,
		`- <https://appliedgo.net/di/>`,
	)

	ma.HandleStatic("pointer performance", "learn more about pointers and their impact on performance", nil,
		`The answer to whether using a pointer offers a performance gain is complex and is not always the case. Please read these posts for more information:`,
		`- <https://medium.com/@vCabbage/go-are-pointers-a-performance-optimization-a95840d3ef85>`,
		`- <https://segment.com/blog/allocation-efficiency-in-high-performance-go-services/>`,
	)

	ma.HandleStatic("gopath", "learn more about using the GOPATH", []string{"gopath problem", "issue with gopath", "help with gopath"},
		"Your project should be structured as follows:",
		"```GOPATH=~/go",
		"~/go/src/sourcecontrol/username/project/```",
		"Whilst you _can_ get around the GOPATH, it's ill-advised. Read more about the GOPATH here: https://github.com/golang/go/wiki/GOPATH",
	)

	ma.HandleStatic("playground", "info on sharing Go code via the Go Playground", []string{"go playground", "goplay", "goplay space"},
		`Please share your code via the Go Playground. The Playground offers a specialized runtime environment, that others can use to read your code, iterate on it, and share it back with you:`,
		`- <https://go.dev/play/>`,
		`There is also this stylized alternate front-end: <https://goplay.space/>`,
	)

	ma.HandleStatic("code of conduct", "info about the code of conduct", []string{"coc"},
		`We're all expected to follow the GoBridge Code of Conduct, which is itself a superset of the Go Community Code of Conduct. You can find both here:`,
		`- <http://coc.golangbridge.org>`,
		`- <https://golang.org/conduct>`,
		`If you have any questions or concerns please reach out in <#C4U9J9QBT> or email support@gobridge.org.`,
	)

	ma.HandleStatic("screenshots", "why you shouldn't use screenshots", []string{"screenshot"},
		`Screenshots are neither easy to read nor very accessible for many people. Please consider copying code, errors, or command output into a Slack snippet, the <https://go.dev/play/|Go Playground>, or a <https://gist.github.com/|GitHub Gist>.`,
	)

	ma.HandleStatic("go tour", "link to the Go tour", []string{"tour"},
		`The Go Tour is a great resource for learning Go, and can be found here: <https://tour.golang.org/welcome/1>`,
	)

	ma.HandleStatic("doesn't work", "x doesn't work", []string{"more context", "doesnt work", "doesntwork"},
		`Can you share more context on what you expected and what you saw instead?`,
		`If there's an error, are you able to provide it in full and share how you generated that error?`,
	)

	ma.HandleStatic("ask", "how to ask questions", []string{"don't ask", "dont ask", "dontask", "just ask", "justask"},
		`Don't ask to ask. Just ask. We'll let you know if there's a better place to ask.`,
		`- <https://dontasktoask.com/>`,
	)

	ma.HandleStatic("crosspost", "cross-posting to multiple channels", []string{"xpost"},
		`Please keep your questions to a single channel. If you don't get a reply in a while, then consider cross-posting.`,
	)

	injectFyneMessageResponses(ma)
}

func injectFyneMessageResponses(ma *handler.MessageActions) {
	ma.HandleStatic("fyne books", "returns a list of books that cover the Fyne toolkit", nil,
		`Here are some helpful books that discuss Fyne:`,
		`- Andrew Williams Building Cross-Platform GUI Applications with Fyne <https://www.packtpub.com/product/building-cross-platform-gui-applications-with-fyne/9781800563162>`,
		`- Andrew Williams Hands-On GUI Application Development in Go <https://www.packtpub.com/product/hands-on-gui-application-development-in-go/9781789138412>`,
	)

	ma.HandleStatic("fyne tour", "link to the Fyne tour", nil,
		`The Fyne Tour is a starting point for learning the toolkit, and can be found here: <https://tour.fyne.io/>`,
	)

	ma.HandleStatic("fyne compile", "tips for common Fyne compile issues", []string{"fyne cgo", "fyne windows"},
		`To compile a Fyne app you will need CGo working which requires a C compiler as well as Go.
			For more information follow the platform specific hints at <https://developer.fyne.io/started/>.
			Windows based developers may need to double check that their C compiler is 64bit as many default to 32`,
	)

	ma.HandleStatic("fyne docs", "link to the Fyne documentation", []string{"fyne doc"},
		`There are various sources of Fyne documentation depending on what you are looking for:`,
		`- Getting started <https://developer.fyne.io/started/>`,
		`- API documentation <https://developer.fyne.io/api/>`,
		`- Tutorials and tips for advanced topics <https://developer.fyne.io/tutorial/>`,
		`- Video tutorials and conference recordings <https://www.youtube.com/c/fyne-io>`,
	)

	ma.HandleStatic("fyne layouts", "information about Fyne layout handling", []string{"fyne layout"},
		`In Fyne, a containers layout controls the size and position of widgets,
			there is a list of standard layouts at <https://developer.fyne.io/started/layouts>
			and you can build your own <https://developer.fyne.io/tutorial/custom-layout>`,
	)

	ma.HandleStatic("fyne sizes", "information about widget sizing in Fyne", []string{"fyne size"},
		`In Fyne, a widgets size and position is controlled by the layout of the container that it is inside <https://developer.fyne.io/faq/layout>.
		If your widget is too small perhaps consider avoiding use of VBox and HBox which try to pack items in small.`,
	)

	ma.HandleStatic("fyne theme", "links for information about Fyne themes", []string{"fyne themes", "fyne style", "fyne styles"},
		`The look and feel of a Fyne application is controlled by the current theme.
			The default material design theme has both light and dark modes (set by user preference)
			Apps can specify a custom theme <https://developer.fyne.io/tutorial/custom-theme>,
			more info about customisation at <https://developer.fyne.io/faq/theme>`,
	)
}

const newbieResourcesMessage = `First you should take the language tour: <https://tour.golang.org/>

Then, you should visit:
 - <https://golang.org/doc/code.html> to learn how to organize your Go workspace
 - <https://golang.org/doc/effective_go.html> be more effective at writing Go
 - <https://golang.org/ref/spec> learn more about the language itself
 - <https://golang.org/doc/#articles> a lot more reading material

There are some awesome websites as well:
 - <https://blog.gopheracademy.com> great resources for Gophers in general
 - <http://gotime.fm> awesome weekly podcast of Go awesomeness
 - <https://gobyexample.com> examples of how to do things in Go
 - <http://go-database-sql.org> how to use SQL databases in Go
 - <https://dmitri.shuralyov.com/idiomatic-go> tips on how to write more idiomatic Go code
 - <https://divan.github.io/posts/avoid_gotchas> will help you avoid gotchas in Go
 - <https://golangbot.com> tutorials to help you get started in Go
 - <https://tutorialedge.net> a collection of articles around various aspects of Go

There's also an exhaustive list of videos <http://gophervids.appspot.com> related to Go from various authors.

If you prefer books, you can try these:
 - <http://www.golangbootcamp.com/book>
 - <http://gopl.io/>
 - <https://www.manning.com/books/go-in-action> (if you e-mail @wkennedy at bill@ardanlabs.com you can get a free copy for being part of this Slack)

If you want to learn how to organize your Go project, make sure to read: <https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1#.ds38va3pp>.
Once you are accustomed to the language and syntax, you can read this series of articles for a walkthrough the various standard library packages: <https://medium.com/go-walkthrough>.

Finally, <https://github.com/golang/go/wiki#learning-more-about-go> will give a list of even more resources to learn Go`
