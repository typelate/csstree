package csstree

import "testing"

// TestTokenName checks that a name token compares by its value, escapes
// decoded, ASCII lowercased, and that a function token keeps its (.
func TestTokenName(t *testing.T) {
	for _, test := range []struct {
		source, want string
	}{
		{"calc(", "calc("},
		{"CALC(", "calc("},
		{`c\61lc(`, "calc("},
		{`\63 alc(`, "calc("},
		{`c\41 LC(`, "calc("},
		{`a\((`, "a(("},
		{"PI", "pi"},
		{`P\49`, "pi"},
		{`-INFINITY`, "-infinity"},
		// only ASCII letters are lowercased
		{"\u0130", "\u0130"},
		{`\212A`, "\u212a"},
		// other tokens are their source text
		{"1PX", "1px"},
		{`'A\42'`, `'a\42'`},
	} {
		tokens := stringToTokens(test.source)
		if len(tokens) != 1 {
			t.Fatalf("stringToTokens(%q) = %d tokens, want 1", test.source, len(tokens))
		}
		if got := tokenName(&tokens[0]); got != test.want {
			t.Errorf("tokenName(%q) = %q, want %q", test.source, got, test.want)
		}
	}
}
