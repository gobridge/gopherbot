package mparser

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func cmpDiff(t *testing.T, thing, diff string) {
	t.Helper()

	if len(diff) > 0 {
		t.Errorf("%s mismatch (-want +got)\n%v", thing, diff)
	}
}

func TestParseAndSplice(t *testing.T) {
	const channelID = "testchan"

	tests := []struct {
		name         string
		input        string
		wantMessage  string
		wantMentions []Mention
	}{
		{
			name: "nothing",
		},
		{
			name:        "no_mentions",
			input:       "this is a message with no mentions",
			wantMessage: "this is a message with no mentions",
		},
		{
			name:        "most_mentions",
			input:       "Hey <@UA1234>, I wanted to let you and <@WZ7890> know that <!subteam^ABC1234> permitted the use of <!everyone> today",
			wantMessage: "Hey , I wanted to let you and  know that  permitted the use of  today",
			wantMentions: []Mention{
				{ID: "UA1234", Type: TypeUser},
				{ID: "WZ7890", Type: TypeUser},
				{ID: "ABC1234", Type: TypeGroup},
				{ID: channelID, Type: TypeEveryone},
			},
		},
		{
			name:        "all_mentions",
			input:       "Hey <@UA1234>, I wanted to let you and <@WZ7890> know that <!subteam^ABC1234> permitted the use of <!channel> in <#CTST123|general> today",
			wantMessage: "Hey , I wanted to let you and  know that  permitted the use of  in  today",
			wantMentions: []Mention{
				{ID: "UA1234", Type: TypeUser},
				{ID: "WZ7890", Type: TypeUser},
				{ID: "ABC1234", Type: TypeGroup},
				{ID: channelID, Type: TypeChannel},
				{ID: "CTST123", Label: "general", Type: TypeChannelRef},
			},
		},
		{
			name:        "channel_no_label",
			input:       "Hey <@UA1234>, welcome to <#CTST123>!",
			wantMessage: "Hey , welcome to !",
			wantMentions: []Mention{
				{ID: "UA1234", Type: TypeUser},
				{ID: "CTST123", Type: TypeChannelRef},
			},
		},
		{
			name:        "random_garbage",
			input:       "<!UW#|^><@>heythere<!^><#><!><@U><@W><#C|g>",
			wantMessage: "<!UW#|^><@>heythere<!^><#><!><@U><@W><#C|g>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, gotMen := ParseAndSplice(tt.input, channelID)

			cmpDiff(t, "mentions", cmp.Diff(tt.wantMentions, gotMen))

			// these strings may be complex, makes it easier to read...
			cmpDiff(t, "message", cmp.Diff(tt.wantMessage, gotMsg))
		})
	}
}
