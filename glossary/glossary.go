// Package glossary provides some common terms that might be handy for a Gopher
// to know.
package glossary

import (
	"fmt"
	"strings"

	"github.com/gobridge/gopherbot/handler"
	"github.com/gobridge/gopherbot/workqueue"
)

// Prefix is the prefix that's intended to be used by the handler.
const Prefix = "define "

// Terms represents the glossary.
type Terms struct {
	entries map[string][]string
	aliases map[string]string
	prefix  string
}

// New generates a new set of glossary terms, from those it returns Terms.
func New(prefix string) Terms {
	t := &Terms{
		entries: make(map[string][]string),
		aliases: make(map[string]string),
		prefix:  prefix,
	}

	for _, tfn := range terms {
		tfn(t)
	}

	return *t
}

// DefineHandler satisfiees handler.MessageActionFn. It handles finding definitions for specific terms.
func (t Terms) DefineHandler(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
	if !m.BotMentioned() {
		return nil
	}

	term := m.Text()[len(t.prefix):]

	// this probably isn't possible with how Slack sends messages
	// but let's have it just in case...
	if len(term) == 0 {
		return r.RespondTo(ctx, "You need to specify a term to define")
	}

	lterm := strings.ToLower(term)
	lt := lterm

	if v, ok := t.aliases[lterm]; ok {
		lt = v
	}

	d, ok := t.entries[lt]
	if !ok {
		return r.RespondTo(ctx, "I'm sorry, I don't have a definition for that.")
	}

	ds := strings.Join(d, "\n")

	var msg string
	if lt != lterm { // alias was used
		msg = fmt.Sprintf("`%s`, or `%s`, is %s", lt, lterm, ds)
	} else {
		msg = fmt.Sprintf("`%s` is %s", lt, ds)
	}

	return r.RespondMentions(ctx, msg)
}
