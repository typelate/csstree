// Ported from lib/__tests/lexer-match-property.js.

package csstree

import (
	"errors"
	"sync"
	"testing"
)

func getMatch(lexer *Lexer, property, value, syntax string) matchOutcome {
	if syntax != "" {
		return outcome(lexer.MatchString(syntax, value))
	}

	return outcome(lexer.MatchPropertyString(property, value))
}

func assertMatchNoError(t *testing.T, call string, match matchOutcome) {
	t.Helper()
	if match.Matched == nil {
		t.Errorf("%s matched = nil, want a match", call)
	}
	if match.err != nil {
		t.Errorf("%s error = %v, want nil", call, match.err)
	}
}

func assertSyntaxMatchError(t *testing.T, call string, match matchOutcome) {
	t.Helper()
	if match.Matched != nil {
		t.Errorf("%s matched a value, want nil", call)
	}
	var matchError *SyntaxMatchError
	if !errors.As(match.err, &matchError) {
		t.Errorf("%s error = %v, want a *SyntaxMatchError", call, match.err)
	}
}

func TestLexerMatchProperty(t *testing.T) {
	lexer := DefaultLexer()
	bar := sync.OnceValue(func() Node {
		return mustParse(t, "bar", ParseOptions{Context: "value"})
	})
	qux := sync.OnceValue(func() Node {
		return mustParse(t, "qux", ParseOptions{Context: "value"})
	})
	customSyntax := sync.OnceValue(func() *Syntax {
		return mustFork(t, &Config{
			Properties: map[string]string{
				"foo":      "bar",
				"-baz-foo": "qux",
			},
		})
	})
	customCSSWideKeywords := sync.OnceValue(func() *Syntax {
		return mustFork(t, &Config{
			CSSWideKeywords: []string{"test"},
		})
	})

	t.Run("vendor prefixes and hacks", func(t *testing.T) {
		t.Run("vendor prefix", func(t *testing.T) {
			match := outcome(customSyntax().Lexer().MatchProperty("-vendor-foo", bar()))

			assertMatchNoError(t, `MatchProperty("-vendor-foo", bar)`, match)
		})

		t.Run("hacks", func(t *testing.T) {
			match := outcome(customSyntax().Lexer().MatchProperty("_foo", bar()))

			assertMatchNoError(t, `MatchProperty("_foo", bar)`, match)
		})

		t.Run("vendor prefix and hack", func(t *testing.T) {
			match := outcome(customSyntax().Lexer().MatchProperty("_-vendor-foo", bar()))

			assertMatchNoError(t, `MatchProperty("_-vendor-foo", bar)`, match)
		})

		t.Run("case insensetive with vendor prefix and hack", func(t *testing.T) {
			for _, property := range []string{"FOO", "-VENDOR-Foo", "_FOO", "_-VENDOR-Foo"} {
				match := outcome(customSyntax().Lexer().MatchProperty(property, bar()))
				assertMatchNoError(t, `MatchProperty("`+property+`", bar)`, match)
			}
		})

		t.Run("should use verdor version first", func(t *testing.T) {
			match := outcome(customSyntax().Lexer().MatchProperty("-baz-foo", qux()))
			assertMatchNoError(t, `MatchProperty("-baz-foo", qux)`, match)

			match = outcome(customSyntax().Lexer().MatchProperty("-baz-baz-foo", qux()))
			if match.Matched != nil {
				t.Errorf(`MatchProperty("-baz-baz-foo", qux) matched a value, want nil`)
			}
			assertErrorMessage(t, `MatchProperty("-baz-baz-foo", qux) error`, match.err, "Unknown property `-baz-baz-foo`")
		})
	})

	t.Run("custom property", func(t *testing.T) {
		match := outcome(lexer.MatchProperty("--foo", bar()))

		if match.Matched != nil {
			t.Errorf(`MatchProperty("--foo", bar) matched a value, want nil`)
		}
		assertErrorMessage(t, `MatchProperty("--foo", bar) error`, match.err, "Lexer matching doesn't applicable for custom properties")
	})

	t.Run("should match css wide keywords", func(t *testing.T) {
		for _, keyword := range testCSSWideKeywords() {
			match := outcome(lexer.MatchPropertyString("color", keyword))

			assertMatchNoError(t, `MatchPropertyString("color", "`+keyword+`")`, match)
		}
	})

	t.Run("should match css wide keywords", func(t *testing.T) {
		for _, keyword := range append(testCSSWideKeywords(), "test") {
			match := outcome(customCSSWideKeywords().Lexer().MatchPropertyString("color", keyword))

			assertMatchNoError(t, `MatchPropertyString("color", "`+keyword+`")`, match)
		}
	})

	t.Run("should not be matched to empty value", func(t *testing.T) {
		match := outcome(lexer.MatchProperty("color", mustParse(t, "", ParseOptions{Context: "value", Positions: true})))

		assertNotMatchedAt(t, `MatchProperty("color", "")`, match, 1, 1)
	})

	forEachDefinitionSyntaxTest(t, func(t *testing.T, testType, testState, name string, lexer *Lexer, property, value, syntax string) {
		switch testType {
		case "valid":
			runDefinitionSyntaxValue(t, testState, name, func(t *testing.T) {
				match := getMatch(lexer, property, value, syntax)

				// temporary solution to avoid var() using errors
				if match.err != nil {
					if errors.Is(match.err, ErrVarNotSupported) || errors.Is(match.err, ErrCustomProperty) {
						return
					}
				}

				if match.Matched == nil {
					t.Errorf("match(%q, %q, %q) = nil, want a match: %v", property, syntax, value, match.err)
				}
			})

		case "invalid":
			runDefinitionSyntaxValue(t, testState, name, func(t *testing.T) {
				match := getMatch(lexer, property, value, syntax)

				if match.Matched != nil {
					t.Errorf("should NOT MATCH to %q", value)
				}
				var matchError *SyntaxMatchError
				if !errors.As(match.err, &matchError) {
					t.Errorf("match(%q, %q, %q) error = %v, want a *SyntaxMatchError", property, syntax, value, match.err)
				}
			})
		}
	})
}

// assertNotMatchedAt checks a "Mismatch" error located at line:column.
func assertNotMatchedAt(t *testing.T, call string, match matchOutcome, line, column int) {
	t.Helper()
	if match.Matched != nil {
		t.Errorf("%s matched a value, want nil", call)
	}
	var matchError *SyntaxMatchError
	if !errors.As(match.err, &matchError) {
		t.Fatalf("%s error = %v, want a *SyntaxMatchError", call, match.err)
	}
	if matchError.RawMessage != "Mismatch" {
		t.Errorf("%s error.RawMessage = %q, want %q", call, matchError.RawMessage, "Mismatch")
	}
	if matchError.Line != line || matchError.Column != column {
		t.Errorf("%s error line:column = %d:%d, want %d:%d", call, matchError.Line, matchError.Column, line, column)
	}
}
