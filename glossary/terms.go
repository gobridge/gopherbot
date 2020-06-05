package glossary

import "fmt"

var terms = []gOption{
	// terms represents all the terms known by the glossary
	//
	// when adding items, please order alphabetically by the term

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
