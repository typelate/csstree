package csstree

import (
	"testing"
)

// TestLexerUnicodeLowerCase checks that keywords, units and custom idents
// are lowercased as JavaScript's toLowerCase does: U+0130 LATIN CAPITAL
// LETTER I WITH DOT ABOVE lowercases to "i\u0307" (i and U+0307), not to an
// ASCII "i", so it never matches an ASCII keyword or unit.
func TestLexerUnicodeLowerCase(t *testing.T) {
	runMatchCases(t, DefaultLexer(), []matchCase{
		// Enum (a | b | c of keywords)
		{kind: "property", name: "display", value: "\u0130nline"},
		{kind: "property", name: "float", value: "r\u0130ght"},
		// <custom-ident> excludes only the ASCII CSS-wide keywords and default
		{kind: "type", name: "custom-ident", value: "\u0130nherit", matched: true},
		{kind: "type", name: "custom-ident", value: "d\u0130fault", matched: true},
		{kind: "type", name: "custom-ident", value: "inherit"},
		// the CSS-wide keywords
		{kind: "property", name: "color", value: "\u0130nherit"},
		// units
		{kind: "property", name: "width", value: "1\u0130n"},
		{kind: "type", name: "length", value: "1\u0130N"},
		{kind: "syntax", name: "<attr-unit>", value: "\u0130n"},
		// U+212A KELVIN SIGN lowercases to an ASCII k in JavaScript, but
		// units are ASCII case-insensitive (the port follows the spec)
		{kind: "type", name: "frequency", value: "1\u212Ahz"},
	})
}

// TestLexerKeywordHackLineTerminator checks that dropping the \0 and \9
// hacks from a keyword stops at a line terminator, as JavaScript's
// /\\[09].*$/ does: "." matches no U+2028 or U+2029 and "$" is the end of
// the value.
func TestLexerKeywordHackLineTerminator(t *testing.T) {
	runMatchCases(t, DefaultLexer(), []matchCase{
		{kind: "property", name: "color", value: "red\\9\u2028x"},
		{kind: "property", name: "color", value: "red\\9\u2028x", ast: true},
		{kind: "property", name: "color", value: "red\\0\u2029"},
		{kind: "syntax", name: "a | b", value: "a\\9\u2028"},
		{kind: "syntax", name: "a", value: "a\\9\u2028"},
		{kind: "property", name: "color", value: "red\\9", matched: true},
		{kind: "property", name: "color", value: "red\\9x", matched: true},
	})
}

// TestFindFragmentsEmptyMatch checks that searching the fragments of a
// syntax that matched no tokens (an empty value of a property that allows
// one) fails rather than panics: upstream throws a TypeError reading the
// first token of the empty match.
func TestFindFragmentsEmptyMatch(t *testing.T) {
	lexer := DefaultLexer()
	value := mustParse(t, "", ParseOptions{Context: "value"})

	if result := lexer.MatchProperty("-webkit-box-reflect", value); result.Matched == nil {
		t.Fatalf("MatchProperty(-webkit-box-reflect, empty) error = %v, want a match", result.Error)
	}

	fragments, err := lexer.FindValueFragments("-webkit-box-reflect", value, "Property", "-webkit-box-reflect")
	if err == nil {
		t.Errorf("FindValueFragments(-webkit-box-reflect, empty) = %v, want an error", fragments)
	}

	ast := mustParse(t, "a{-webkit-box-reflect:}", ParseOptions{})
	fragments, err = lexer.FindAllFragments(ast, "Property", "-webkit-box-reflect")
	if err == nil {
		t.Errorf("FindAllFragments(-webkit-box-reflect, empty) = %v, want an error", fragments)
	}

	// other syntaxes of the empty match are searched as before
	fragments, err = lexer.FindValueFragments("-webkit-box-reflect", value, "Type", "length")
	if err != nil || len(fragments) != 0 {
		t.Errorf("FindValueFragments(-webkit-box-reflect, empty, <length>) = %v, %v, want none", fragments, err)
	}
}

// TestLexerSpecEdgeCases matches values from the edge cases of CSS Values
// and Units 4 and CSS Syntax 3. The iteration counts pinned for the
// combinators, multipliers and a few at-rule preludes are upstream's, to
// keep the matcher's backtracking the same. The expectations are upstream's results,
// except where a comment says the port follows the spec; elsewhere, where
// upstream disagrees with the spec, the comment says what the spec expects
// (the port keeps upstream's behaviour).
func TestLexerSpecEdgeCases(t *testing.T) {
	const mismatch = "Mismatch"

	runMatchCases(t, DefaultLexer(), []matchCase{
		// Values 4 §2.2 component value combinators: juxtaposition binds
		// tighter than &&, && than ||, || than |
		{kind: "syntax", name: "a b | c", value: "a b", matched: true, iterations: 6},
		{kind: "syntax", name: "a b | c", value: "c", matched: true, iterations: 6},
		{kind: "syntax", name: "a b | c", value: "a c", iterations: 8, mismatchOffset: 2, errText: mismatch},
		{kind: "syntax", name: "a || b && c", value: "c b a", matched: true, iterations: 16},
		{kind: "syntax", name: "a || b && c", value: "b a c", iterations: 13, mismatchOffset: 2},
		{kind: "syntax", name: "a || b && c", value: "a b", iterations: 22},
		{kind: "syntax", name: "a | b || c", value: "b c", matched: true, iterations: 10},
		{kind: "syntax", name: "a | b || c", value: "a c", iterations: 11, mismatchOffset: 2},

		// Values 4 §2.4 multipliers and the ! required group
		{kind: "syntax", name: "[a? b?]!", value: "", iterations: 12},
		{kind: "syntax", name: "[a? b?]!", value: "b", matched: true, iterations: 11},
		{kind: "syntax", name: "<length>#{1,3}", value: "1px, 2px, 3px, 4px", iterations: 28, mismatchOffset: 13},
		{kind: "syntax", name: "<length>#{1,3}", value: "1px, 2px, 3px", matched: true, iterations: 27},
		{kind: "syntax", name: "<length>#{2,}", value: "1px", iterations: 17},
		{kind: "syntax", name: "<length>{2}", value: "1px 2px 3px", iterations: 11, mismatchOffset: 8},
		{kind: "syntax", name: "<length>+#", value: "1px 2px, 3px", matched: true, iterations: 49},
		{kind: "syntax", name: "<length>#", value: "1px,", iterations: 14},
		{kind: "syntax", name: "<length>#", value: "1px,,2px", iterations: 20, mismatchOffset: 4},

		// Values 4 §5.1 bracketed range notation; upstream does not check
		// ranges with units (a FIXME), so <angle [0deg,360deg]> takes
		// 400deg and -1deg; the port follows the spec
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "400deg"},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "-1deg"},
		{kind: "syntax", name: "<length [0,∞]>", value: "-1px"},
		{kind: "syntax", name: "<length [0,∞]>", value: "calc(-1px)", matched: true},
		{kind: "syntax", name: "<integer [1,∞]>", value: "0"},
		{kind: "syntax", name: "<integer [1,∞]>", value: "calc(0)", matched: true},
		{kind: "syntax", name: "<number [0,1]>", value: "1e309"},
		{kind: "syntax", name: "<number [0,∞]>", value: "1e309", matched: true},
		{kind: "syntax", name: "<number [-∞,0]>", value: "-1e309", matched: true},
		{kind: "syntax", name: "<number [0,1]>", value: "-0", matched: true},

		// Values 4 §4.2 <custom-ident> excludes the CSS-wide keywords and
		// default in any ASCII case, compared by their value: upstream
		// compares the source text, so takes \69nherit (inherit); the port
		// follows the spec
		{kind: "type", name: "custom-ident", value: "DEFAULT"},
		{kind: "type", name: "custom-ident", value: "Revert-Layer"},
		{kind: "type", name: "custom-ident", value: "\\69nherit"},
		{kind: "type", name: "custom-ident", value: "\\64 efault"},
		// keywords are compared by their value too: \72 ed is red, r\ed is
		// r followed by U+00ED
		{kind: "property", name: "color", value: "r\\ed"},
		{kind: "property", name: "color", value: "\\72 ed", matched: true},
		// Values 4 §4.2 <dashed-ident>, CSS Variables <custom-property-name>
		{kind: "type", name: "dashed-ident", value: "--", matched: true},
		{kind: "type", name: "dashed-ident", value: "-a"},
		{kind: "type", name: "custom-property-name", value: "--"},

		// Values 4 §5 numbers: <integer> is a number-token of digits
		{kind: "type", name: "integer", value: "1.0"},
		{kind: "type", name: "integer", value: "1e3"},
		{kind: "type", name: "integer", value: "-0", matched: true},
		{kind: "type", name: "integer", value: "+0", matched: true},
		{kind: "type", name: "number", value: "+.5", matched: true},
		{kind: "type", name: "number", value: "1e3", matched: true},
		{kind: "type", name: "number", value: "1e309", matched: true},
		// a <number> is a <number-token>: upstream does not check the token
		// type, so it takes any token whose text is a number, the delims +
		// and - and idents like e5; the port follows the spec
		{kind: "type", name: "number", value: "+"},
		{kind: "type", name: "number", value: "-"},
		{kind: "type", name: "number", value: "e5"},
		{kind: "type", name: "number", value: "-E3"},
		{kind: "property", name: "opacity", value: "e5"},
		{kind: "property", name: "opacity", value: "+"},
		{kind: "property", name: "line-height", value: "e5"},
		// Values 4 §6 unitless zero is a <length>, not an <angle> or <time>
		{kind: "type", name: "length", value: "0", matched: true},
		{kind: "type", name: "length", value: "0.0", matched: true},
		{kind: "type", name: "length", value: "-0", matched: true},
		{kind: "type", name: "length", value: "1"},
		{kind: "type", name: "angle", value: "0"},
		{kind: "type", name: "time", value: "0"},

		// Values 4 §10.9 math functions are type checked (upstream
		// consumes them without checking their arguments, so calc(red) is
		// a <length> and hypot(1px) a <number> there); a math function
		// resolving to a <number> is taken for an <integer>
		{kind: "property", name: "width", value: "calc(red)"},
		{kind: "type", name: "number", value: "hypot(1px)"},
		{kind: "type", name: "integer", value: "calc(1.5)", matched: true},

		// CSS Color 4 <hex-color>
		{kind: "type", name: "hex-color", value: "#ABCDEF", matched: true},
		{kind: "type", name: "hex-color", value: "#abcde"},

		// properties: !important is not part of a value, vendor and hack
		// prefixes fall back to the property, custom properties and var()
		// are not matched
		{kind: "property", name: "color", value: "red !important", mismatchOffset: 4},
		{kind: "property", name: "*zoom", value: "1", matched: true},
		{kind: "property", name: "_color", value: "red", matched: true},
		{kind: "property", name: "-webkit-color", value: "red", matched: true},
		{kind: "property", name: "COLOR", value: "red", matched: true},
		{kind: "property", name: "--x", value: "red", errText: ErrCustomProperty.Error()},
		{kind: "property", name: "color", value: "var(--x)", errText: ErrVarNotSupported.Error()},
		{kind: "property", name: "color", value: "rgb(VAR(--x))", errText: ErrVarNotSupported.Error()},

		// at-rules: descriptors take no CSS-wide keywords
		{kind: "descriptor", parent: "font-face", name: "font-display", value: "inherit"},
		{kind: "descriptor", parent: "font-face", name: "unicode-range", value: "U+0-7F, U+0400-04FF", matched: true},
		{kind: "descriptor", parent: "property", name: "syntax", value: "'<length>'", matched: true},
		{kind: "descriptor", parent: "property", name: "inherits", value: "false", matched: true},
		{kind: "descriptor", parent: "counter-style", name: "system", value: "fixed 3", matched: true},
		{kind: "descriptor", parent: "page", name: "size", value: "A4 landscape", matched: true},
		{kind: "prelude", name: "media", value: "screen and (min-width: 100px)", matched: true, iterations: 562},
		{kind: "prelude", name: "layer", value: "", matched: true},
		{kind: "prelude", name: "page", value: "", matched: true},
		{kind: "prelude", name: "font-face", value: "x", errText: "At-rule `@font-face` should not contain a prelude"},
		{kind: "prelude", name: "supports", value: "", errText: "At-rule `@supports` should contain a prelude"},
		{kind: "prelude", name: "supports", value: "(display: grid) and (not (display: inline-grid))", matched: true, iterations: 249},
		{kind: "prelude", name: "import", value: "url(a.css) layer(x) supports(display: grid) screen", matched: true, iterations: 550},
		{kind: "prelude", name: "property", value: "--x", matched: true},
		{kind: "prelude", name: "property", value: "x"},
		// css-contain-3 excludes none from <container-name>, and
		// css-counter-styles none and decimal from @counter-style names;
		// mdn-data's syntaxes do not
		{kind: "prelude", name: "container", value: "sidebar (min-width: 400px)", matched: true},
		{kind: "prelude", name: "container", value: "none (min-width: 400px)", matched: true},
		{kind: "prelude", name: "counter-style", value: "none", matched: true},
		{kind: "prelude", name: "counter-style", value: "decimal", matched: true},
		{kind: "prelude", name: "counter-style", value: "inherit"},
	})
}

// TestLexerIterationLimit checks that matching stops after 15000
// iterations with upstream's reason, for a syntax whose alternatives
// backtrack exponentially.
func TestLexerIterationLimit(t *testing.T) {
	const syntax = "[a | a a]* b"

	runMatchCases(t, DefaultLexer(), []matchCase{
		{kind: "syntax", name: syntax, value: "a a a a a a a a a a a a a c", iterations: 14037, mismatchOffset: 26, errText: "Mismatch"},
		{kind: "syntax", name: syntax, value: "a a a a a a a a a a a a a a c", iterations: 15000, mismatchOffset: 28, errText: exitReasonIterationLimit},
	})
}
