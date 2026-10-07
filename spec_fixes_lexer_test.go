package csstree

import (
	"errors"
	"testing"
)

// specMatchCase is a value matched against a property, a type or a
// definition syntax, with whether the spec says it matches.
type specMatchCase struct {
	kind  string // "property", "type" or "syntax"
	name  string
	value string
	want  bool
}

func runSpecMatchCases(t *testing.T, cases []specMatchCase) {
	t.Helper()

	lexer := DefaultLexer()
	for _, c := range cases {
		var (
			result *MatchResult
			call   string
		)
		switch c.kind {
		case "property":
			result, call = lexer.MatchPropertyString(c.name, c.value), "MatchPropertyString"
		case "type":
			result, call = lexer.MatchTypeString(c.name, c.value), "MatchTypeString"
		case "syntax":
			result, call = lexer.MatchString(c.name, c.value), "MatchString"
		default:
			t.Fatalf("unknown kind %q", c.kind)
		}

		if got := result.Matched != nil; got != c.want {
			t.Errorf("%s(%q, %q) = %v, want %v (error: %v)", call, c.name, c.value, got, c.want, result.Error)
		}
	}
}

// TestSpecEscapedKeywords checks that keywords, units and function names
// are compared by the value of the identifier with its escapes decoded,
// ASCII case-insensitively (CSS Syntax 3 §4.3.11 consume a name, §8
// "Tokens match if their value is a match"; CSS Values 4 §2.1, §4).
func TestSpecEscapedKeywords(t *testing.T) {
	runSpecMatchCases(t, []specMatchCase{
		// keywords (Keyword and Enum)
		// \ed is the hex escape of U+00ED (í), so r\ed is rí, not red
		{"property", "color", `r\ed`, false},
		{"property", "color", `r\65 d`, true},
		{"property", "color", `r\45 D`, true},
		{"property", "color", `\72 ed`, true},
		{"property", "color", `\52 ED`, true},
		{"property", "color", `re\64`, true},
		{"property", "color", `\000072 ed`, true},
		{"property", "display", `\62lock`, true},
		{"property", "display", `\62 lock`, true},
		{"property", "display", `\62 lock\9`, true},
		{"syntax", "a", `\61`, true},
		{"syntax", "a", `\41`, true},
		{"syntax", "a | b", `\62`, true},
		{"syntax", "a", `\62`, false},
		{"property", "color", `'red'`, false},
		{"property", "color", `r\65 d x`, false},
		// the \0 and \9 hacks are still dropped
		{"property", "color", `red\9`, true},
		{"property", "color", `red\0`, true},
		// CSS-wide keywords
		{"property", "color", `\69nherit`, true},
		{"property", "color", `\49 NITIAL`, true},
		// ASCII case-insensitive only: U+212A KELVIN SIGN is not k
		{"property", "display", "blocK", false},
		{"property", "display", `bloc\212A`, false},
		{"syntax", "k", "K", false},
		// units
		{"property", "width", `1p\x`, true},
		{"property", "width", `1\70 x`, true},
		{"property", "width", `1P\58`, true},
		{"property", "width", `1p\x\9`, true},
		{"property", "width", `1px\9`, true},
		{"property", "width", `1p\y`, false},
		{"type", "frequency", "1Khz", false},
		{"type", "frequency", `1\212A hz`, false},
		{"type", "frequency", `1\6B hz`, true},
		{"syntax", "<attr-unit>", `p\x`, true},
		{"syntax", "<attr-unit>", `\25`, false}, // an ident %, not the delim %
		{"syntax", "<attr-unit>", "Khz", false},
		// function names
		{"property", "color", `\72 gb(0 0 0)`, true},
		{"property", "color", `RG\42 (0 0 0)`, true},
		{"property", "width", `c\61lc(1px)`, true},
		{"property", "width", `\63 alc(1px)`, true},
		{"syntax", "fn( <number> )", `f\6e(1)`, true},
		{"syntax", "fn( <number> )", `f\6f(1)`, false},
		// at-keywords
		{"syntax", "@foo", `@f\6fo`, true},
		{"syntax", "@foo", `@f\6f`, false},
		// <custom-ident> excludes the CSS-wide keywords and default after
		// decoding
		{"type", "custom-ident", `\69nherit`, false},
		{"type", "custom-ident", `\64 efault`, false},
		{"type", "custom-ident", `\44 EFAULT`, false},
		{"type", "custom-ident", `unse\74`, false},
		{"type", "custom-ident", `\66oo`, true},
		{"type", "custom-ident", `inherit\9`, true},
		// <dashed-ident> and <custom-property-name> check the decoded value
		{"type", "dashed-ident", `\2d-foo`, true},
		{"type", "dashed-ident", `-\2d foo`, true},
		{"type", "dashed-ident", `\2d foo`, false},
		{"type", "custom-property-name", `\2d-foo`, true},
		{"type", "custom-property-name", `-\2d`, false},
		{"type", "custom-property-name", `\2d\2d`, false},
	})
}

// TestSpecEscapedVar checks that var() is found by its decoded name.
func TestSpecEscapedVar(t *testing.T) {
	lexer := DefaultLexer()
	for _, value := range []string{`v\61r(--x)`, `\76 AR(--x)`, `rgb(V\41R(--x))`} {
		result := lexer.MatchPropertyString("color", value)
		if !errors.Is(result.Error, ErrVarNotSupported) {
			t.Errorf("MatchPropertyString(%q, %q) error = %v, want %v", "color", value, result.Error, ErrVarNotSupported)
		}
	}
}
