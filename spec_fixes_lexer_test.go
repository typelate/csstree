package csstree

import (
	"errors"
	"testing"
)

// TestSpecEscapedKeywords checks that keywords, units and function names
// are compared by the value of the identifier with its escapes decoded,
// ASCII case-insensitively (CSS Syntax 3 §4.3.11 consume a name, §8
// "Tokens match if their value is a match"; CSS Values 4 §2.1, §4).
func TestSpecEscapedKeywords(t *testing.T) {
	runMatchCases(t, DefaultLexer(), []matchCase{
		// keywords (Keyword and Enum)
		// \ed is the hex escape of U+00ED (í), so r\ed is rí, not red
		{kind: "property", name: "color", value: `r\ed`},
		{kind: "property", name: "color", value: `r\65 d`, matched: true},
		{kind: "property", name: "color", value: `r\45 D`, matched: true},
		{kind: "property", name: "color", value: `\72 ed`, matched: true},
		{kind: "property", name: "color", value: `\52 ED`, matched: true},
		{kind: "property", name: "color", value: `re\64`, matched: true},
		{kind: "property", name: "color", value: `\000072 ed`, matched: true},
		{kind: "property", name: "display", value: `\62lock`, matched: true},
		{kind: "property", name: "display", value: `\62 lock`, matched: true},
		{kind: "property", name: "display", value: `\62 lock\9`, matched: true},
		{kind: "syntax", name: "a", value: `\61`, matched: true},
		{kind: "syntax", name: "a", value: `\41`, matched: true},
		{kind: "syntax", name: "a | b", value: `\62`, matched: true},
		{kind: "syntax", name: "a", value: `\62`},
		{kind: "property", name: "color", value: `'red'`},
		{kind: "property", name: "color", value: `r\65 d x`},
		// the \0 and \9 hacks are still dropped
		{kind: "property", name: "color", value: `red\9`, matched: true},
		{kind: "property", name: "color", value: `red\0`, matched: true},
		// CSS-wide keywords
		{kind: "property", name: "color", value: `\69nherit`, matched: true},
		{kind: "property", name: "color", value: `\49 NITIAL`, matched: true},
		// ASCII case-insensitive only: U+212A KELVIN SIGN is not k
		{kind: "property", name: "display", value: "blocK"},
		{kind: "property", name: "display", value: `bloc\212A`},
		{kind: "syntax", name: "k", value: "K"},
		// units
		{kind: "property", name: "width", value: `1p\x`, matched: true},
		{kind: "property", name: "width", value: `1\70 x`, matched: true},
		{kind: "property", name: "width", value: `1P\58`, matched: true},
		{kind: "property", name: "width", value: `1p\x\9`, matched: true},
		{kind: "property", name: "width", value: `1px\9`, matched: true},
		{kind: "property", name: "width", value: `1p\y`},
		{kind: "type", name: "frequency", value: "1Khz"},
		{kind: "type", name: "frequency", value: `1\212A hz`},
		{kind: "type", name: "frequency", value: `1\6B hz`, matched: true},
		{kind: "syntax", name: "<attr-unit>", value: `p\x`, matched: true},
		{kind: "syntax", name: "<attr-unit>", value: `\25`}, // an ident %, not the delim %
		{kind: "syntax", name: "<attr-unit>", value: "Khz"},
		// function names
		{kind: "property", name: "color", value: `\72 gb(0 0 0)`, matched: true},
		{kind: "property", name: "color", value: `RG\42 (0 0 0)`, matched: true},
		{kind: "property", name: "width", value: `c\61lc(1px)`, matched: true},
		{kind: "property", name: "width", value: `\63 alc(1px)`, matched: true},
		{kind: "syntax", name: "fn( <number> )", value: `f\6e(1)`, matched: true},
		{kind: "syntax", name: "fn( <number> )", value: `f\6f(1)`},
		// at-keywords
		{kind: "syntax", name: "@foo", value: `@f\6fo`, matched: true},
		{kind: "syntax", name: "@foo", value: `@f\6f`},
		// <custom-ident> excludes the CSS-wide keywords and default after
		// decoding
		{kind: "type", name: "custom-ident", value: `\69nherit`},
		{kind: "type", name: "custom-ident", value: `\64 efault`},
		{kind: "type", name: "custom-ident", value: `\44 EFAULT`},
		{kind: "type", name: "custom-ident", value: `unse\74`},
		{kind: "type", name: "custom-ident", value: `\66oo`, matched: true},
		{kind: "type", name: "custom-ident", value: `inherit\9`, matched: true},
		// an escaped backslash is part of the value: \\initial is \initial
		{kind: "type", name: "custom-ident", value: `\\initial`, matched: true},
		{kind: "type", name: "custom-ident", value: `\\default`, matched: true},
		// <dashed-ident> and <custom-property-name> check the decoded value
		{kind: "type", name: "dashed-ident", value: `\2d-foo`, matched: true},
		{kind: "type", name: "dashed-ident", value: `-\2d foo`, matched: true},
		{kind: "type", name: "dashed-ident", value: `\2d foo`},
		{kind: "type", name: "custom-property-name", value: `\2d-foo`, matched: true},
		{kind: "type", name: "custom-property-name", value: `-\2d`},
		{kind: "type", name: "custom-property-name", value: `\2d\2d`},
	})
}

// TestSpecEscapedVar checks that var() is found by its decoded name.
func TestSpecEscapedVar(t *testing.T) {
	lexer := DefaultLexer()
	for _, value := range []string{`v\61r(--x)`, `\76 AR(--x)`, `rgb(V\41R(--x))`} {
		_, err := lexer.MatchPropertyString("color", value)
		if !errors.Is(err, ErrVarNotSupported) {
			t.Errorf("MatchPropertyString(%q, %q) error = %v, want %v", "color", value, err, ErrVarNotSupported)
		}
	}
}

// TestSpecNumberTokens checks that <number> is a <number-token> and
// <integer> a <number-token> of digits (CSS Values 4 §5.2, §5.3): delims
// and idents whose text reads as a number are not numbers.
func TestSpecNumberTokens(t *testing.T) {
	runMatchCases(t, DefaultLexer(), []matchCase{
		{kind: "type", name: "number", value: "+"},
		{kind: "type", name: "number", value: "-"},
		{kind: "type", name: "number", value: "e5"},
		{kind: "type", name: "number", value: "-E3"},
		{kind: "type", name: "number", value: "e-5"},
		{kind: "type", name: "number", value: "+e5"},
		{kind: "property", name: "opacity", value: "e5"},
		{kind: "property", name: "opacity", value: "+"},
		{kind: "property", name: "line-height", value: "e5"},
		{kind: "syntax", name: "<number [0,∞]>", value: "e5"},
		{kind: "type", name: "number", value: "1", matched: true},
		{kind: "type", name: "number", value: "+.5", matched: true},
		{kind: "type", name: "number", value: "-1e-3", matched: true},
		{kind: "type", name: "number", value: "1e309", matched: true},
		{kind: "property", name: "opacity", value: ".5", matched: true},
		{kind: "property", name: "line-height", value: "1.5", matched: true},
		// the IE postfix hack is still taken
		{kind: "type", name: "number", value: `1\9`, matched: true},
		{kind: "type", name: "number", value: `1\0`, matched: true},
		{kind: "type", name: "number", value: `1px`},
		{kind: "type", name: "number", value: `1%`},
		// an integer is written with digits only
		{kind: "type", name: "integer", value: "1", matched: true},
		{kind: "type", name: "integer", value: "-1", matched: true},
		{kind: "type", name: "integer", value: "+0", matched: true},
		{kind: "type", name: "integer", value: "1.0"},
		{kind: "type", name: "integer", value: "1e3"},
		{kind: "type", name: "integer", value: "1E0"},
		{kind: "type", name: "integer", value: "e5"},
		{kind: "type", name: "integer", value: "+"},
		{kind: "property", name: "z-index", value: "1e3"},
		{kind: "property", name: "z-index", value: "1.0"},
		{kind: "property", name: "z-index", value: "10", matched: true},
	})
}

// TestSpecRangesWithUnits checks the bracketed range notation of CSS
// Values 4 §5.1 for bounds with units: values of a unit that converts to
// the unit of the bound are compared in a canonical unit, and relative
// units are compared only by sign.
func TestSpecRangesWithUnits(t *testing.T) {
	runMatchCases(t, DefaultLexer(), []matchCase{
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "0deg", matched: true},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "360deg", matched: true},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "360DEG", matched: true},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "400deg"},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "-1deg"},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "-0deg", matched: true},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "1turn", matched: true},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "1.01turn"},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "400grad", matched: true},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "401grad"},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "6.28rad", matched: true},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: "6.3rad"},
		{kind: "syntax", name: "<angle [0deg,360deg]>", value: `400d\65 g`},
		{kind: "syntax", name: "<time [0s,∞]>", value: "0s", matched: true},
		{kind: "syntax", name: "<time [0s,∞]>", value: "1ms", matched: true},
		{kind: "syntax", name: "<time [0s,∞]>", value: "-1s"},
		{kind: "syntax", name: "<time [0s,∞]>", value: "-1ms"},
		{kind: "syntax", name: "<time [0s,∞]>", value: "1e309s", matched: true},
		{kind: "syntax", name: "<time [100ms,1s]>", value: "0.1s", matched: true},
		{kind: "syntax", name: "<time [100ms,1s]>", value: "99ms"},
		{kind: "syntax", name: "<time [100ms,1s]>", value: "1000ms", matched: true},
		{kind: "syntax", name: "<time [100ms,1s]>", value: "1001ms"},
		{kind: "syntax", name: "<time [100ms,1s]>", value: "1s", matched: true},
		{kind: "property", name: "animation-duration", value: "-1s"},
		{kind: "property", name: "animation-duration", value: "1s, -1ms"},
		{kind: "property", name: "animation-duration", value: "1s, 0ms", matched: true},
		{kind: "syntax", name: "<length [0,1in]>", value: "96px", matched: true},
		{kind: "syntax", name: "<length [0,1in]>", value: "97px"},
		{kind: "syntax", name: "<length [0,1in]>", value: "2.54cm", matched: true},
		{kind: "syntax", name: "<length [0,1in]>", value: "2.6cm"},
		{kind: "syntax", name: "<length [0,1in]>", value: "25.4mm", matched: true},
		{kind: "syntax", name: "<length [0,1in]>", value: "101.6q", matched: true},
		{kind: "syntax", name: "<length [0,1in]>", value: "102Q"},
		{kind: "syntax", name: "<length [0,1in]>", value: "72pt", matched: true},
		{kind: "syntax", name: "<length [0,1in]>", value: "73pt"},
		{kind: "syntax", name: "<length [0,1in]>", value: "6pc", matched: true},
		{kind: "syntax", name: "<length [0,1in]>", value: "7pc"},
		{kind: "syntax", name: "<length [0,1in]>", value: "0", matched: true},
		// relative lengths are not comparable at parse time, except by
		// sign: every unit is a non-negative multiple of the canonical one
		{kind: "syntax", name: "<length [0,1in]>", value: "100em", matched: true},
		{kind: "syntax", name: "<length [0,1in]>", value: "100vw", matched: true},
		{kind: "syntax", name: "<length [0,1in]>", value: "-1em"},
		{kind: "syntax", name: "<length [1px,∞]>", value: "1px", matched: true},
		{kind: "syntax", name: "<length [1px,∞]>", value: "0.5px"},
		{kind: "syntax", name: "<length [1px,∞]>", value: "0"},
		{kind: "syntax", name: "<length [1px,∞]>", value: "0em"},
		{kind: "syntax", name: "<length [1px,∞]>", value: "-1em"},
		{kind: "syntax", name: "<length [1px,∞]>", value: "0.01em", matched: true},
		{kind: "syntax", name: "<length [1em,2em]>", value: "1.5em", matched: true},
		{kind: "syntax", name: "<length [1em,2em]>", value: "3em"},
		{kind: "syntax", name: "<length [1em,2em]>", value: "3px", matched: true},
		{kind: "syntax", name: "<length [0,∞]>", value: "-1px"},
		{kind: "syntax", name: "<length [0,∞]>", value: "-1vw"},
		{kind: "syntax", name: "<length-percentage [0,∞]>", value: "-1%"},
		{kind: "syntax", name: "<resolution [0dpi,∞]>", value: "-1x"},
		{kind: "syntax", name: "<resolution [0dpi,∞]>", value: "0x", matched: true},
		{kind: "syntax", name: "<resolution [1dppx,192dpi]>", value: "1x", matched: true},
		{kind: "syntax", name: "<resolution [1dppx,192dpi]>", value: "96dpi", matched: true},
		{kind: "syntax", name: "<resolution [1dppx,192dpi]>", value: "95dpi"},
		{kind: "syntax", name: "<resolution [1dppx,192dpi]>", value: "37.8dpcm", matched: true},
		{kind: "syntax", name: "<resolution [1dppx,192dpi]>", value: "37.7dpcm"},
		{kind: "syntax", name: "<resolution [1dppx,192dpi]>", value: "2dppx", matched: true},
		{kind: "syntax", name: "<resolution [1dppx,192dpi]>", value: "2.1dppx"},
		{kind: "syntax", name: "<frequency [0hz,1khz]>", value: "1000Hz", matched: true},
		{kind: "syntax", name: "<frequency [0hz,1khz]>", value: "1001hz"},
		{kind: "syntax", name: "<frequency [0hz,1khz]>", value: "-1khz"},
		{kind: "syntax", name: "<flex [0,∞]>", value: "-1fr"},
		{kind: "syntax", name: "<flex [1fr,∞]>", value: "0.5fr"},
		{kind: "syntax", name: "<percentage [0,100]>", value: "101%"},
		{kind: "syntax", name: "<dimension [0px,1in]>", value: "97px"},
		{kind: "syntax", name: "<dimension [0px,1in]>", value: "1foo", matched: true},
		{kind: "syntax", name: "<dimension [0px,1in]>", value: "-1foo"},
		// the IE postfix hack is not part of the unit
		{kind: "syntax", name: "<time [0s,∞]>", value: `-1s\9`},
		{kind: "syntax", name: "<time [0s,∞]>", value: `1s\9`, matched: true},
	})
}

// TestSpecURange checks <urange> against CSS Syntax 3 §7.1 (Candidate
// Recommendation Draft, 24 December 2021): the first token is an ident u,
// the representations of the other tokens are interpreted as a range, and
// the range is invalid when its end is above U+10FFFF or below its start.
// Upstream checks the token forms and digit counts only, so it takes
// ranges the spec rejects, such as U+FF-00, U+1-zz and ua+1.
func TestSpecURange(t *testing.T) {
	runMatchCases(t, DefaultLexer(), []matchCase{
		{kind: "type", name: "urange", value: "U+0-10FFFF", matched: true},
		{kind: "type", name: "urange", value: "U+10FFFF", matched: true},
		{kind: "type", name: "urange", value: "U+110000"},
		{kind: "type", name: "urange", value: "U+0-110000"},
		{kind: "type", name: "urange", value: "U+10FFF?", matched: true},
		{kind: "type", name: "urange", value: "U+11????"},
		{kind: "type", name: "urange", value: "U+??????"},
		{kind: "type", name: "urange", value: "U+0?????", matched: true},
		{kind: "type", name: "urange", value: "U+FF-00"},
		{kind: "type", name: "urange", value: "U+FF-FF", matched: true},
		{kind: "type", name: "urange", value: "U+1-zz"},
		{kind: "type", name: "urange", value: "u+a-"},
		{kind: "type", name: "urange", value: "U+1e-3"},
		{kind: "type", name: "urange", value: "U+1e-1e", matched: true},
		{kind: "type", name: "urange", value: "U+1e3", matched: true},
		{kind: "type", name: "urange", value: "u+12e-130", matched: true},
		{kind: "type", name: "urange", value: "ua+1"},
		{kind: "type", name: "urange", value: "Ua+1"},
		{kind: "type", name: "urange", value: "UU+1"},
		{kind: "type", name: "urange", value: `\75+1`, matched: true},
		{kind: "type", name: "urange", value: `\55 +1`, matched: true},
		{kind: "type", name: "urange", value: "U+1?-2"},
		{kind: "type", name: "urange", value: "u+?", matched: true},
		{kind: "type", name: "urange", value: "U+0-7F", matched: true},
		{kind: "type", name: "urange", value: "u+0025-00FF", matched: true},
		{kind: "type", name: "urange", value: "U+4??", matched: true},
		{kind: "type", name: "urange", value: "U+1-1234567"},
		{kind: "type", name: "urange", value: "U+??-1", mismatchOffset: 4},
		{kind: "syntax", name: "<urange>#", value: "U+0-7F, U+110000"},
		{kind: "property", name: "unicode-range", value: "U+0-7F"}, // a descriptor, not a property
		{kind: "descriptor", parent: "font-face", name: "unicode-range", value: "U+0-7F, U+0400-04FF", matched: true},
		{kind: "descriptor", parent: "font-face", name: "unicode-range", value: "U+0-7F, U+FF-00"},
		{kind: "descriptor", parent: "font-face", name: "unicode-range", value: "U+110000"},
	})
}

// TestSpecAnPlusBOddEven checks where odd and even of the An+B grammar
// (CSS Syntax 3 §6.2) match. <an+b>, mdn-data's syntax of the spec's
// production, takes them, in any ASCII case and with escapes. The generic
// <an-plus-b> is the production of the AnPlusB node, which holds A and B
// only (the parser keeps odd and even in an Identifier of the Nth node),
// so it leaves odd and even to the syntaxes using it, as upstream does.
func TestSpecAnPlusBOddEven(t *testing.T) {
	runMatchCases(t, DefaultLexer(), []matchCase{
		{kind: "type", name: "an+b", value: "odd", matched: true},
		{kind: "type", name: "an+b", value: "EVEN", matched: true},
		{kind: "type", name: "an+b", value: `\6f dd`, matched: true},
		{kind: "type", name: "an+b", value: "2n+1", matched: true},
		{kind: "type", name: "an-plus-b", value: "2n+1", matched: true},
		{kind: "type", name: "an-plus-b", value: "odd"},
		{kind: "type", name: "an-plus-b", value: "even"},
		{kind: "syntax", name: "<an-plus-b> | odd | even", value: "odd", matched: true},
		{kind: "syntax", name: "<an-plus-b> | odd | even", value: "Even", matched: true},
	})
}

// TestSpecNullInHash checks that a NUL in a hash token reads as U+FFFD
// REPLACEMENT CHARACTER (CSS Syntax 3 §3.3 preprocessing), a name code
// point, as the tokenizer reads it: #\x00a is an <id-selector>.
func TestSpecNullInHash(t *testing.T) {
	runMatchCases(t, DefaultLexer(), []matchCase{
		{kind: "type", name: "id-selector", value: "#\x00a", matched: true},
		{kind: "type", name: "id-selector", value: "#a\x00", matched: true},
		{kind: "type", name: "hex-color", value: "#\x00ab"},
	})
}
