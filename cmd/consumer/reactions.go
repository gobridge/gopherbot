package main

import "github.com/gobridge/gopherbot/handler"

func injectMessageReactions(r *handler.MessageActions) {
	r.HandleReaction("bbq", "bbqgopher")
	r.HandleReaction("ghost", "ghost")
	r.HandleReaction("spacex", "rocket")
	r.HandleReaction("spacemacs", "spacemacs")
	r.HandleReaction("my adorable little gophers", "gopher")

	r.HandleReaction("dragon", "dragon")
	r.HandleReaction("dargon", "dragon")
	r.HandleReaction("ermergerd", "dragon")
	r.HandleReaction("ermahgerd", "dragon")

	r.HandleReaction("beer me", "beer", "beers")

	r.HandleMentionedReaction("thank", "gopher")
	r.HandleMentionedReaction("cheers", "gopher")
	r.HandleMentionedReaction("hello", "gopher")
	r.HandleMentionedReaction("wave", "wave", "gopher")

	r.HandleReactionRand("vim", "emacs")
	r.HandleReactionRand("emacs", "vim")

	r.HandleStaticContains("︵", "┬─┬ノ( º _ ºノ)")
	r.HandleStaticContains("彡", "┬─┬ノ( º _ ºノ)")

	r.HandleStaticContains("proverbs", `<https://go-proverbs.github.io>`)
}
