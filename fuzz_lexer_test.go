package csstree

import (
	"strings"
	"testing"

	"github.com/typelate/csstree/tokenizer"
)

// FuzzMatchProperty matches arbitrary values, as text and as parsed ASTs,
// against properties of the default lexer: it must never panic, and a
// mismatch must come with an error.
func FuzzMatchProperty(f *testing.F) {
	for _, seed := range []struct{ property, value string }{
		{"color", "red"},
		{"margin", "1px auto 0 -2em"},
		{"background", "url(x.png) no-repeat 0 0 / cover, linear-gradient(red, blue)"},
		{"font", "bold 12px/1.5 Arial, sans-serif"},
		{"grid-template-columns", "[a] repeat(3, minmax(10px, 1fr)) [b]"},
		{"unicode-range", "U+0025-00FF, u+4??"},
		{"width", "calc(100% - 2 * var(--x))"},
		{"transition", "opacity 1s ease-in 2s, transform .5s"},
		{"color", "rgb(1 2 3 / 50%)"},
		{"line-height", "0"},
		{"--custom", "anything"},
		{"unknown", "1px"},
	} {
		f.Add(seed.property, seed.value)
	}

	f.Fuzz(func(t *testing.T, property, value string) {
		lexer := DefaultLexer()

		checkResult := func(call string, result *MatchResult, err error) {
			switch {
			case result == nil:
				t.Errorf("%s = a nil result, error %v", call, err)
			case result.Matched == nil && err == nil:
				t.Errorf("%s = no match and no error", call)
			case result.Matched != nil && err != nil:
				t.Errorf("%s = a match and error %v", call, err)
			}
		}

		result, err := lexer.MatchPropertyString(property, value)
		checkResult("MatchPropertyString", result, err)

		ast, err := Parse(value, ParseOptions{Context: "value", Positions: true})
		if err != nil {
			return
		}
		result, err = lexer.MatchProperty(property, ast)
		checkResult("MatchProperty", result, err)
		if _, err := lexer.FindValueFragments(property, ast, "Type", "color"); err != nil {
			t.Errorf("FindValueFragments() error = %v", err)
		}
	})
}

// FuzzMathFunction matches arbitrary math functions against properties
// taking numeric types: it must never panic, a mismatch must come with an
// error, and nesting a valid math function in calc() keeps it valid.
func FuzzMathFunction(f *testing.F) {
	for _, seed := range []struct{ property, value string }{
		{"width", "calc(100% / 3 - 2 * 1em - 2 * 1px)"},
		{"width", "clamp(none, 5%, 100px)"},
		{"width", "round(line-width, 1.5px)"},
		{"width", "calc(1px * pow(10% / 1px, 2))"},
		{"width", "calc((1px + 2px) / (1 + 1))"},
		{"width", "calc(1px + env(x))"},
		{"opacity", "calc(50% / 1%)"},
		{"rotate", "atan2(1px, 1%)"},
		{"z-index", "round(down, 1.5, 1)"},
		{"line-height", "calc(1em + 10%)"},
		{"grid-template-columns", "repeat(2, calc(10% + 1px))"},
		{"transition-duration", "min(1s, 500ms), 2s"},
		// found by fuzzing (testdata/fuzz/FuzzMathFunction): an unclosed
		// calc() with a substitution function in an unclosed block,
		// which is taken unchecked and is not one closed math function
		{"width", "CAlC([env()"},
		{"width", "CAlC(env({"},
	} {
		f.Add(seed.property, seed.value)
	}

	f.Fuzz(func(t *testing.T, property, value string) {
		lexer := DefaultLexer()
		result, err := lexer.MatchPropertyString(property, value)
		if (result.Matched == nil) == (err == nil) {
			t.Fatalf("MatchPropertyString(%q, %q) matched = %t, error = %v", property, value, result.Matched != nil, err)
		}

		if err != nil || !isSingleMathFunction(value) || strings.Count(value, "(") >= maxCalcDepth/2 {
			return
		}
		nested := "calc(" + value + ")"
		if _, err := lexer.MatchPropertyString(property, nested); err != nil {
			t.Errorf("MatchPropertyString(%q, %q) matched, but not %q: %v", property, value, nested, err)
		}
	})
}

// isSingleMathFunction reports whether value is one math function closed
// by its ) and nothing else, without substitution functions.
func isSingleMathFunction(value string) bool {
	tokens := stringToTokens(value)
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Function || !isMathFunctionName(tokenName(&tokens[0])) {
		return false
	}
	if tokens[len(tokens)-1].typ != tokenizer.RightParenthesis {
		return false
	}
	for _, token := range tokens {
		// a function with substitution functions is matched as upstream
		if token.typ == tokenizer.Function && isSubstitutionFunctionName(tokenName(&token)) {
			return false
		}
	}
	return consumeFunction(tokens) == len(tokens)
}
