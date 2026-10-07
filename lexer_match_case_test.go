package csstree

import (
	"errors"
	"testing"
)

// matchCase is a match of a value (CSS text, or parsed as a value or an
// at-rule prelude when ast is set) against a property, type, definition
// syntax, at-rule prelude or at-rule descriptor of a lexer.
type matchCase struct {
	kind  string // "property", "type", "syntax", "prelude" or "descriptor"
	name  string
	value string

	ast    bool
	parent string // the at-rule of a descriptor

	matched bool
	// iterations, when not 0, is the number of iterations of the matcher.
	// Only the cases that pin the matcher's parity with upstream set it.
	iterations int
	// mismatchOffset, when not 0, is the mismatch offset of a
	// SyntaxMatchError.
	mismatchOffset int
	// errText, when set, is the text of the error (the raw message of a
	// SyntaxMatchError).
	errText string
}

// String describes the case for the name of its subtest.
func (c matchCase) String() string {
	desc := c.kind + " " + c.name + ": " + c.value
	if c.ast {
		desc += " (ast)"
	}
	return desc
}

func (c matchCase) run(t *testing.T, lexer *Lexer) matchOutcome {
	t.Helper()

	if c.ast {
		context := ContextValue
		if c.kind == "prelude" {
			context = ContextAtrulePrelude
		}
		node := mustParse(t, c.value, ParseOptions{Context: context, Atrule: c.name, Positions: true})
		switch c.kind {
		case "property":
			return outcome(lexer.MatchProperty(c.name, node))
		case "type":
			return outcome(lexer.MatchType(c.name, node))
		case "syntax":
			return outcome(lexer.Match(c.name, node))
		case "prelude":
			return outcome(lexer.MatchAtrulePrelude(c.name, node))
		case "descriptor":
			return outcome(lexer.MatchAtruleDescriptor(c.parent, c.name, node))
		}
	}

	switch c.kind {
	case "property":
		return outcome(lexer.MatchPropertyString(c.name, c.value))
	case "type":
		return outcome(lexer.MatchTypeString(c.name, c.value))
	case "syntax":
		return outcome(lexer.MatchString(c.name, c.value))
	case "prelude":
		return outcome(lexer.MatchAtrulePreludeString(c.name, c.value))
	case "descriptor":
		return outcome(lexer.MatchAtruleDescriptorString(c.parent, c.name, c.value))
	}

	t.Fatalf("unknown kind %q", c.kind)
	return matchOutcome{}
}

// runMatchCases runs each case as a subtest against lexer.
func runMatchCases(t *testing.T, lexer *Lexer, cases []matchCase) {
	t.Helper()

	for _, c := range cases {
		t.Run(c.String(), func(t *testing.T) {
			result := c.run(t, lexer)

			if got := result.Matched != nil; got != c.matched {
				t.Fatalf("matched = %v, want %v (error: %v)", got, c.matched, result.err)
			}
			if c.iterations != 0 && result.Iterations != c.iterations {
				t.Errorf("iterations = %d, want %d", result.Iterations, c.iterations)
			}
			if c.matched {
				if result.err != nil {
					t.Errorf("error = %v, want nil", result.err)
				}
				return
			}

			var matchErr *SyntaxMatchError
			isMatchErr := errors.As(result.err, &matchErr)
			if c.mismatchOffset != 0 && (!isMatchErr || matchErr.MismatchOffset != c.mismatchOffset) {
				t.Errorf("error = %v, want a mismatch at offset %d", result.err, c.mismatchOffset)
			}
			if c.errText != "" {
				text := ""
				switch {
				case isMatchErr:
					text = matchErr.RawMessage
				case result.err != nil:
					text = result.err.Error()
				}
				if text != c.errText {
					t.Errorf("error = %q, want %q", text, c.errText)
				}
			}
		})
	}
}

// parsedMatchCases returns cases matching their values parsed, which
// match as their text does.
func parsedMatchCases(cases []matchCase) []matchCase {
	parsed := make([]matchCase, len(cases))
	for i, c := range cases {
		c.ast = true
		parsed[i] = c
	}
	return parsed
}

// matchOutcome is what a Match method returns, for the tests that pass
// the result and the error to a helper together or keep them in a table.
type matchOutcome struct {
	*MatchResult
	err error
}

func outcome(result *MatchResult, err error) matchOutcome {
	return matchOutcome{MatchResult: result, err: err}
}
