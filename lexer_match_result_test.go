// Ported from lib/__tests/lexer-match-result.js.

package csstree

import (
	"slices"
	"testing"

	"github.com/typelate/csstree/definitionsyntax"
)

// formatTrace formats a trace as upstream's deep-equal objects would print:
// "<type> <name>" for each syntax.
func formatTrace(trace []MatchSyntax) []string {
	if trace == nil {
		return nil
	}
	result := []string{}
	for _, syntax := range trace {
		result = append(result, syntax.Type()+" "+syntaxName(syntax))
	}
	return result
}

func TestLexerMatchResult(t *testing.T) {
	lexer := DefaultLexer()

	// upstream beforeEach
	setup := func(t *testing.T) (testNode Node, match, mismatch matchOutcome) {
		ast := mustParse(t, "rgb(1, 2, 3)", ParseOptions{Context: "value"})
		testNode = ast.(*Value).Children.First().(*Function).Children.First()
		match = outcome(lexer.MatchProperty("background", ast))
		mismatch = outcome(lexer.MatchProperty("margin", ast))
		return testNode, match, mismatch
	}

	t.Run("getTrace()", func(t *testing.T) {
		testNode, match, mismatch := setup(t)

		got := formatTrace(match.Trace(testNode))
		want := []string{
			"Property background",
			"Type final-bg-layer",
			"Property background-color",
			"Type color",
			"Type color-base",
			"Type color-function",
			"Type rgb()",
			"Type number",
		}
		if !slices.Equal(got, want) {
			t.Errorf("Trace() = %q, want %q", got, want)
		}
		// upstream also compares opts: null of each Type
		for _, syntax := range match.Trace(testNode) {
			if node, ok := syntax.(*definitionsyntax.Type); ok && node.Opts != nil {
				t.Errorf("Trace() %v has opts, want nil", syntax)
			}
		}
		if trace := mismatch.Trace(testNode); trace != nil {
			t.Errorf("mismatch.Trace() = %v, want nil", trace)
		}
	})

	t.Run("isType()", func(t *testing.T) {
		testNode, match, mismatch := setup(t)

		for _, test := range []struct {
			result matchOutcome
			typ    string
			want   bool
		}{
			{match, "color", true},
			{match, "final-bg-layer", true},
			{match, "background-color", false},
			{match, "foo", false},
			{mismatch, "color", false},
		} {
			if got := test.result.IsType(testNode, test.typ); got != test.want {
				t.Errorf("IsType(node, %q) = %v, want %v", test.typ, got, test.want)
			}
		}
	})

	t.Run("isProperty()", func(t *testing.T) {
		testNode, match, mismatch := setup(t)

		for _, test := range []struct {
			result   matchOutcome
			property string
			want     bool
		}{
			{match, "color", false},
			{match, "final-bg-layer", false},
			{match, "background-color", true},
			{match, "foo", false},
			{mismatch, "color", false},
		} {
			if got := test.result.IsProperty(testNode, test.property); got != test.want {
				t.Errorf("IsProperty(node, %q) = %v, want %v", test.property, got, test.want)
			}
		}
	})

	t.Run("isKeyword()", func(t *testing.T) {
		ast := mustParse(t, "repeat 0", ParseOptions{Context: "value"})
		keywordNode := ast.(*Value).Children.First()
		numberNode := ast.(*Value).Children.Last()
		match := outcome(lexer.MatchProperty("background", ast))

		if !match.IsKeyword(keywordNode) {
			t.Errorf("IsKeyword(repeat) = false, want true")
		}
		if match.IsKeyword(numberNode) {
			t.Errorf("IsKeyword(0) = true, want false")
		}
	})
}
