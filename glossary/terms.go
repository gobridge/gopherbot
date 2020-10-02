package glossary

import "fmt"

var terms = []gOption{
	// terms represents all the terms known by the glossary
	//
	// the bot responds with: <TERM>, or <ALIAS>, is <DEFINITION>
	//
	// when adding items, please order alphabetically by the term

	define("domain-driven design", []string{"ddd", "domain-driven development", "domain driven design"},
		`a concept around how to structure your source code around business domain(s).`,
		`See <https://en.wikipedia.org/wiki/Domain-driven_design> for more info.`,
	),

	define("dependency injection", []string{"di"},
		`a technique in which a type or function receives other things that it depends on, such as a database handler or logger`,
		``,
		"Note: my `dependency injection` command provides more details on how to use dependency injection in Go.",
	),

	define("test-driven development", []string{"tdd", "test-driven development", "test driven development"},
		`a concept around writing tests first followed by just enough code to satisfy the test and, eventually, refactoring`,
		`See <https://en.wikipedia.org/wiki/Test-driven_development> for more info.`,
	),

	define("variadic", []string{"variadic", "variadic parameter", "variadic function"},
		`a concept describing the use of a parameter type in a function signature which may occur zero to many times.`,
		``,
		`Note: the ellipsis (...) is used to denote a variadic (e.g. parameter ...string) and it is the last parameter in the signature.`
	),

	define("blank identifier", []string{"blank", "blank identifier", "underscore"},
		`an indicator that something is not used. in a for loop, as an example, the index may be ignored when it is not needed (for _, val...).`,
		``,
		`Note: when used with a package name, the blank identifier allows the Go compiler to execute the init function but does not require the package to be called.`,
		`  This is a common practice for packages such as database drivers.`,
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
