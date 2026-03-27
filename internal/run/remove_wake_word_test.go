package run

import "testing"

func TestRemoveWakeWord(t *testing.T) {
	cases := []struct {
		text    string
		word    string
		aliases []string
		expect  string
	}{
		{"clawd make it so", "clawd", nil, "make it so"},
		{"clawd, launch torpedo", "clawd", nil, "launch torpedo"},
		{"no wake here", "clawd", nil, "no wake here"},
		{"Claude engage", "clawd", []string{"claude"}, "engage"},
		// Multi-word wake phrases
		{"hey computer, refactor this", "hey computer", nil, "refactor this"},
		{"Hey Computer do something", "hey computer", nil, "do something"},
		{"Hey computer, refactor this.", "hey computer", []string{"claude"}, "refactor this."},
		{"Claude, help me out", "hey computer", []string{"claude"}, "help me out"},
	}
	for _, c := range cases {
		got := removeWakeWord(c.text, c.word, c.aliases)
		if got != c.expect {
			t.Fatalf("removeWakeWord(%q)=%q want %q", c.text, got, c.expect)
		}
	}
}
