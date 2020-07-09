package glossary

import "fmt"

var terms = []gOption{
	// terms represents all the terms known by the glossary
	//
	// the bot responds with: <TERM>, or <ALIAS>, is <DEFINITION>
	//
	// when adding items, please order alphabetically by the term

	define("domain-driven design", []string{"ddd", "domain-driven development", "domain driven design"},
		`Domain-driven Design (aka Domain-driven Development) is a concept around how to structure your source code around business domain(s).`,
		`See <https://en.wikipedia.org/wiki/Domain-driven_design> for more info.`,
	),

	define("dependency injection", []string{"di"},
		`a technique in which a type or function receives other things that it depends on, such as a database handler or logger`,
		``,
		"Note: my `dependency injection` command provides more details on how to use dependency injection in Go.",
	),
}

type gOption func(t *Terms)

func define(term string, aliases []string, content ...string) gOption {
	return func(t *Terms) {
		if _, ok := t.entries[term]; ok {
			panic(fmt.Sprintf("term %s already defined", term))
		}

		for _, a := range aliases {
			if v, ok := t.aliases[a]; ok {
				panic(fmt.Sprintf("alias %s already exists to %s", a, v))
			}

			t.aliases[a] = term
		}

		t.entries[term] = content
	}
}
