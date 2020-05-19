package interactivity

func injectMessageReactions(r *MessageActions) {
	r.HandleReaction("bbq", false, false, "bbqgopher")
	r.HandleReaction("ghost", false, false, "ghost")
	r.HandleReaction("spacex", false, false, "rocket")
	r.HandleReaction("buffalo", false, false, "gobuffalo")
	r.HandleReaction("gobuffalo", false, false, "gobuffalo")
	r.HandleReaction("spacemacs", false, false, "spacemacs")
	r.HandleReaction("my adorable little gophers", false, false, "gopher")

	r.HandleReaction("dragon", false, false, "dragon")
	r.HandleReaction("dargon", false, false, "dragon")
	r.HandleReaction("ermergerd", false, false, "dragon")
	r.HandleReaction("ermahgerd", false, false, "dragon")

	r.HandleReaction("beer me", false, false, "beer", "beers")

	r.HandleReaction("vim", false, true, "emacs")
	r.HandleReaction("emacs", false, true, "vim")

	r.HandleReaction("︵", true, false, "┬─┬ノ( º _ ºノ)")
	r.HandleReaction("彡", true, false, "┬─┬ノ( º _ ºノ)")
}
