package csstree

import (
	"slices"
	"strings"
	"testing"
)

// Tests of CSS Values and Units Level 4 §10 math functions, where this port
// type checks calc() and friends instead of consuming any balanced
// function as upstream csstree does.
// https://drafts.csswg.org/css-values-4/#math

func TestSpecMathMatchProperty(t *testing.T) {
	cases := []matchCase{
		// §10.1 calc() takes a <calc-sum>; width is a <length-percentage>
		{kind: "property", name: "width", value: "calc(1px + 2px)", matched: true},
		{kind: "property", name: "width", value: "calc(100% - 2em)", matched: true},
		{kind: "property", name: "width", value: "calc(100% / 3 - 2 * 1em - 2 * 1px)", matched: true},
		{kind: "property", name: "width", value: "calc((1px + 2px) * 3)", matched: true},
		{kind: "property", name: "width", value: "calc(calc(1px + 2px) * 3)", matched: true},
		{kind: "property", name: "width", value: "calc( 1px )", matched: true},
		{kind: "property", name: "width", value: "calc(1px*2)", matched: true},
		{kind: "property", name: "width", value: "calc(1px/2)", matched: true},
		{kind: "property", name: "width", value: "calc(1px /* note */ + 2px)", matched: true},
		{kind: "property", name: "width", value: "calc(10%)", matched: true},
		{kind: "property", name: "width", value: "calc(1px * (2 + 3))", matched: true},
		{kind: "property", name: "width", value: "calc((1px + 2px) / (1 + 1))", matched: true},
		{kind: "property", name: "width", value: "calc(1px / (2px / 1px))", matched: true},
		{kind: "property", name: "width", value: "calc(1px / 1px * 1px)", matched: true},
		{kind: "property", name: "width", value: "calc(10px * 10px / 1px)", matched: true},
		{kind: "property", name: "width", value: "calc(1px / 0)", matched: true},
		{kind: "property", name: "width", value: "calc(min(1px, max(2px, 3%)) + clamp(1px, 2px, 3px))", matched: true},
		// a function name is ASCII case-insensitive
		{kind: "property", name: "width", value: "CALC(1px)", matched: true},
		{kind: "property", name: "width", value: "Calc(1px + 1%)", matched: true},
		{kind: "property", name: "width", value: "-webkit-calc(1px + 2%)", matched: true},
		{kind: "property", name: "width", value: "-moz-calc(1px + 2%)", matched: true},
		// and may be escaped (CSS Syntax 3 §4.3.7), as may keywords and
		// units
		{kind: "property", name: "width", value: `c\61lc(1px)`, matched: true},
		{kind: "property", name: "width", value: `c\61lc(red)`},
		{kind: "property", name: "width", value: `calc(1px * p\69)`, matched: true},
		{kind: "property", name: "width", value: `calc(1p\78)`, matched: true},
		{kind: "property", name: "width", value: `clamp(n\6fne, 1px, none)`, matched: true},
		// §10.2 comparison functions
		{kind: "property", name: "width", value: "min(10px, 5%)", matched: true},
		{kind: "property", name: "width", value: "max(10px, 5%, 1em)", matched: true},
		{kind: "property", name: "width", value: "clamp(10px, 5%, 100px)", matched: true},
		{kind: "property", name: "width", value: "clamp(none, 5%, 100px)", matched: true},
		{kind: "property", name: "width", value: "clamp(10px, 5%, NONE)", matched: true},
		{kind: "property", name: "width", value: "clamp(none, 5%, none)", matched: true},
		// §10.3 stepped value functions
		{kind: "property", name: "width", value: "round(10.5px, 1px)", matched: true},
		{kind: "property", name: "width", value: "round(up, 10.5px, 1px)", matched: true},
		{kind: "property", name: "width", value: "round(TO-ZERO, 10.5px, 1px)", matched: true},
		{kind: "property", name: "width", value: "round(line-width, 1.5px)", matched: true},
		{kind: "property", name: "width", value: "round(line-width, 1.5px, 1px)", matched: true},
		{kind: "property", name: "width", value: "mod(18px, 5px)", matched: true},
		{kind: "property", name: "width", value: "rem(18px, 5%)", matched: true},
		// §10.4 and §10.5 functions of numbers used as factors
		{kind: "property", name: "width", value: "calc(1px * sin(30deg))", matched: true},
		{kind: "property", name: "width", value: "calc(1px * cos(1))", matched: true},
		{kind: "property", name: "width", value: "calc(1px * tan(0.5turn))", matched: true},
		{kind: "property", name: "width", value: "calc(1px * pow(2, 3))", matched: true},
		{kind: "property", name: "width", value: "calc(1px * sqrt(4))", matched: true},
		{kind: "property", name: "width", value: "calc(1px * exp(1) / log(2))", matched: true},
		{kind: "property", name: "width", value: "calc(1px * log(8, 2))", matched: true},
		{kind: "property", name: "width", value: "calc(1px * pow(10% / 1px, 2))", matched: true},
		{kind: "property", name: "width", value: "hypot(30px, 40px)", matched: true},
		{kind: "property", name: "width", value: "hypot(30px, 4%)", matched: true},
		// §10.6 sign-related functions
		{kind: "property", name: "width", value: "abs(-1px)", matched: true},
		{kind: "property", name: "width", value: "calc(1px * sign(-5%))", matched: true},
		// §10.7 numeric constants are ASCII case-insensitive <number>s
		{kind: "property", name: "width", value: "calc(1px * e)", matched: true},
		{kind: "property", name: "width", value: "calc(1px * PI)", matched: true},
		{kind: "property", name: "width", value: "calc(infinity * 1px)", matched: true},
		{kind: "property", name: "width", value: "calc(-INFINITY * 1px)", matched: true},
		{kind: "property", name: "width", value: "calc(NaN * 1px)", matched: true},
		{kind: "property", name: "width", value: "calc(nan * 1px)", matched: true},

		// mismatches: the arguments are not a <calc-sum>
		{kind: "property", name: "width", value: "calc(red)"},
		{kind: "property", name: "width", value: "calc()"},
		{kind: "property", name: "width", value: "min()"},
		{kind: "property", name: "width", value: "calc(1px, 2px)"},
		{kind: "property", name: "width", value: "calc(none)"},
		{kind: "property", name: "width", value: "calc(1px +)"},
		{kind: "property", name: "width", value: "calc(+ 1px)"},
		{kind: "property", name: "width", value: "calc(1px *)"},
		{kind: "property", name: "width", value: "calc(1px / )"},
		{kind: "property", name: "width", value: "calc(1px ** 2)"},
		{kind: "property", name: "width", value: "calc(1px 2px)"},
		{kind: "property", name: "width", value: "calc(-(1px))"},
		{kind: "property", name: "width", value: "calc(-pi * 1px)"},
		{kind: "property", name: "width", value: "calc(\"1px\")"},
		{kind: "property", name: "width", value: "calc([1px])"},
		{kind: "property", name: "width", value: "calc(1px + foo(2px))"},
		// §10.8 + and - need whitespace on both sides
		{kind: "property", name: "width", value: "calc(1px+2px)"},
		{kind: "property", name: "width", value: "calc(1px +2px)"},
		{kind: "property", name: "width", value: "calc(1px- 2px)"},
		{kind: "property", name: "width", value: "calc(1px/**/+ 2px)"},
		// §10.2, §10.3 argument counts and keywords
		{kind: "property", name: "width", value: "clamp(1px, 2px)"},
		{kind: "property", name: "width", value: "clamp(1px, none, 2px)"},
		{kind: "property", name: "width", value: "clamp(1px, 2px, 3px, 4px)"},
		{kind: "property", name: "width", value: "round(1.5px)"},
		{kind: "property", name: "width", value: "round(nearest 1px, 1px)"},
		{kind: "property", name: "width", value: "round(sideways, 1px, 1px)"},
		{kind: "property", name: "width", value: "round(up, 1px, 1px, 1px)"},
		{kind: "property", name: "width", value: "round(line-width, 1.5)"},
		{kind: "property", name: "width", value: "mod(1px)"},
		{kind: "property", name: "width", value: "hypot()"},
		// §10.9 type checking
		{kind: "property", name: "width", value: "calc(1px + 2s)"},
		{kind: "property", name: "width", value: "calc(1px - 1px + 1s)"},
		{kind: "property", name: "width", value: "calc(0 * 5px + 10s)"},
		{kind: "property", name: "width", value: "calc(1px * 1px)"},
		{kind: "property", name: "width", value: "calc(1 / 1px)"},
		{kind: "property", name: "width", value: "calc(0)"},
		{kind: "property", name: "width", value: "calc(0 + 5px)"},
		{kind: "property", name: "width", value: "calc(5)"},
		{kind: "property", name: "width", value: "calc(pi)"},
		{kind: "property", name: "width", value: "calc(1px + (2s))"},
		{kind: "property", name: "width", value: "calc(10% * 10%)"},
		{kind: "property", name: "width", value: "min(1px, 1s)"},
		{kind: "property", name: "width", value: "calc(sin(1px) * 1px)"},
		{kind: "property", name: "width", value: "calc(sqrt(4px))"},
		{kind: "property", name: "width", value: "calc(pow(2px, 2))"},
		{kind: "property", name: "width", value: "asin(0.5)"},
		{kind: "property", name: "width", value: "atan2(1px, 2px)"},
		{kind: "property", name: "width", value: "sign(1px)"},
		// <flex> values are not allowed in math functions (CSS Grid 2 §7.2.4)
		{kind: "property", name: "width", value: "calc(1fr)"},

		// opacity is <number> | <percentage>: percentages resolve against a
		// <number> and do not combine with numbers
		{kind: "property", name: "opacity", value: "calc(0.5)", matched: true},
		{kind: "property", name: "opacity", value: "calc(50%)", matched: true},
		{kind: "property", name: "opacity", value: "calc(100% / 3)", matched: true},
		{kind: "property", name: "opacity", value: "calc(50% / 1%)", matched: true},
		{kind: "property", name: "opacity", value: "calc(1 * 1%)", matched: true},
		{kind: "property", name: "opacity", value: "sin(1)", matched: true},
		{kind: "property", name: "opacity", value: "sign(-1)", matched: true},
		{kind: "property", name: "opacity", value: "calc(1px)"},
		{kind: "property", name: "opacity", value: "calc(.25 + 25%)"},
		{kind: "property", name: "opacity", value: "hypot(1px)"},

		// line-height is normal | <number> | <length> | <percentage>
		{kind: "property", name: "line-height", value: "calc(1 + 1)", matched: true},
		{kind: "property", name: "line-height", value: "calc(1em + 10%)", matched: true},
		{kind: "property", name: "line-height", value: "calc(1 + 1em)"},
		{kind: "property", name: "line-height", value: "calc(1 + 10%)"},

		// rotate takes an <angle>
		{kind: "property", name: "rotate", value: "calc(45deg + 0.5turn)", matched: true},
		{kind: "property", name: "rotate", value: "atan2(1, 1)", matched: true},
		{kind: "property", name: "rotate", value: "atan2(1px, 1%)"},
		{kind: "property", name: "rotate", value: "asin(0.5)", matched: true},
		{kind: "property", name: "rotate", value: "calc(1rad * pi)", matched: true},
		{kind: "property", name: "rotate", value: "x calc(90deg)", matched: true},
		{kind: "property", name: "rotate", value: "calc(45deg + 10%)"},
		{kind: "property", name: "rotate", value: "asin(1deg)"},
		{kind: "property", name: "rotate", value: "calc(45)"},
		{kind: "property", name: "rotate", value: "sin(45deg)"},

		// transition-duration takes a <time>
		{kind: "property", name: "transition-duration", value: "calc(1s + 100ms)", matched: true},
		{kind: "property", name: "transition-duration", value: "calc(1s * 2)", matched: true},
		{kind: "property", name: "transition-duration", value: "min(1s, 500ms), 2s", matched: true},
		{kind: "property", name: "transition-duration", value: "calc(1s + 1px)"},
		{kind: "property", name: "transition-duration", value: "calc(2 / 1s)"},
		{kind: "property", name: "transition-duration", value: "calc(10%)"},

		// flex-grow takes a <number>; §10.12 a math function out of the
		// range is clamped, not rejected
		{kind: "property", name: "flex-grow", value: "calc(1 + 1)", matched: true},
		{kind: "property", name: "flex-grow", value: "calc(-1)", matched: true},
		{kind: "property", name: "flex-grow", value: "calc(1px)"},
		{kind: "property", name: "flex-grow", value: "calc(1%)"},

		// §10.9 a math function resolving to <number> can be used where an
		// <integer> is accepted
		{kind: "property", name: "z-index", value: "calc(1 + 1)", matched: true},
		{kind: "property", name: "z-index", value: "calc(1.5)", matched: true},
		{kind: "property", name: "z-index", value: "round(1.5)", matched: true},
		{kind: "property", name: "z-index", value: "round(down, 1.5, 1)", matched: true},
		{kind: "property", name: "z-index", value: "calc(1px)"},
		{kind: "property", name: "z-index", value: "calc(1%)"},
		{kind: "property", name: "counter-increment", value: "foo calc(1 + 1)", matched: true},
		{kind: "property", name: "tab-size", value: "calc(4)", matched: true},
		{kind: "property", name: "tab-size", value: "calc(4px)", matched: true},
		{kind: "property", name: "tab-size", value: "calc(4%)"},

		// font-weight takes a <number [1,1000]>
		{kind: "property", name: "font-weight", value: "calc(400 + 100)", matched: true},
		{kind: "property", name: "font-weight", value: "calc(2000)", matched: true},
		{kind: "property", name: "font-weight", value: "calc(400px)"},
		// font-stretch takes a <percentage>
		{kind: "property", name: "font-stretch", value: "calc(50% + 10%)", matched: true},
		{kind: "property", name: "font-stretch", value: "calc(1)"},
		{kind: "property", name: "font-stretch", value: "calc(1px + 10%)"},

		// margin and padding take <length-percentage>s
		{kind: "property", name: "margin", value: "calc(-10% - 1px)", matched: true},
		{kind: "property", name: "margin", value: "calc(1px) calc(2%)", matched: true},
		{kind: "property", name: "margin", value: "calc(1px + 1s)"},
		{kind: "property", name: "padding", value: "calc(1em + 10%)", matched: true},
		// border-width takes a <length>, not a percentage
		{kind: "property", name: "border-width", value: "calc(2px)", matched: true},
		{kind: "property", name: "border-width", value: "calc(10%)"},
		{kind: "property", name: "border-width", value: "calc(1px + 10%)"},
		{kind: "property", name: "font-size", value: "clamp(1rem, 2.5vw, 2rem)", matched: true},
		{kind: "property", name: "font-size", value: "calc(1em + 1vw)", matched: true},
		{kind: "property", name: "background-position", value: "calc(100% - 10px) calc(100% - 10px)", matched: true},

		// grid tracks: a <flex> is not allowed in a math function
		{kind: "property", name: "grid-template-columns", value: "calc(100px)", matched: true},
		{kind: "property", name: "grid-template-columns", value: "repeat(2, calc(10% + 1px))", matched: true},
		{kind: "property", name: "grid-template-columns", value: "minmax(calc(10px), 1fr)", matched: true},
		{kind: "property", name: "grid-template-columns", value: "calc(1fr)"},
		{kind: "property", name: "grid-template-columns", value: "calc(1fr + 1px)"},

		// aspect-ratio takes a <ratio> of <number>s
		{kind: "property", name: "aspect-ratio", value: "calc(16 / 9)", matched: true},
		{kind: "property", name: "aspect-ratio", value: "16 / calc(9)", matched: true},
		{kind: "property", name: "aspect-ratio", value: "calc(16px / 9)"},

		// math functions in other functions
		{kind: "property", name: "transform", value: "translate(calc(50% + 10px), 0)", matched: true},
		{kind: "property", name: "transform", value: "rotate(calc(1turn / 4))", matched: true},
		{kind: "property", name: "transform", value: "scale(calc(1 + 0.5))", matched: true},
		{kind: "property", name: "transform", value: "scale(calc(1px))"},
		{kind: "property", name: "color", value: "rgb(calc(255 / 2) 0 0)", matched: true},
		{kind: "property", name: "color", value: "rgb(calc(1px) 0 0)"},
		{kind: "property", name: "color", value: "hsl(calc(120deg + 10deg) 50% 50%)", matched: true},

		// arbitrary substitution functions are not type checked, as
		// upstream; anchor-size() and anchor() are <length>s
		{kind: "property", name: "width", value: "calc(1px + env(safe-area-inset-top))", matched: true},
		{kind: "property", name: "width", value: "calc(attr(data-width px))", matched: true},
		{kind: "property", name: "width", value: "calc(1px + anchor-size(width))", matched: true},
		{kind: "property", name: "top", value: "calc(anchor(--a bottom) + 1em)", matched: true},
		{kind: "property", name: "width", value: "calc(1s + anchor-size(width))"},
		{kind: "property", name: "voice-volume", value: "calc(1db + env(x))", matched: true},
		{kind: "property", name: "grid-template-columns", value: "calc(1fr * env(x))", matched: true},
		{kind: "property", name: "voice-volume", value: "calc(1db)"},
	}

	lexer := DefaultLexer()
	runMatchCases(t, lexer, cases)

	// a parsed value is matched as its text is; the parser drops the
	// whitespace of "1px +2px", giving the tokens of "1px+2px", still not a
	// sum
	runMatchCases(t, lexer, parsedMatchCases(cases))
}

func TestSpecMathMatchType(t *testing.T) {
	cases := []matchCase{
		{kind: "type", name: "length", value: "calc(1px)", matched: true},
		{kind: "type", name: "length", value: "calc(1px + 1em)", matched: true},
		{kind: "type", name: "length", value: "calc(10%)"},
		{kind: "type", name: "length", value: "calc(1px + 1%)"},
		{kind: "type", name: "length", value: "asin(0.5)"},
		{kind: "type", name: "length-percentage", value: "calc(10%)", matched: true},
		{kind: "type", name: "length-percentage", value: "calc(1px + 1%)", matched: true},
		{kind: "type", name: "percentage", value: "calc(10% + 1%)", matched: true},
		{kind: "type", name: "percentage", value: "calc(10% + 1px)"},
		{kind: "type", name: "number", value: "calc(1)", matched: true},
		{kind: "type", name: "number", value: "calc(1px / 1px)", matched: true},
		{kind: "type", name: "number", value: "calc(1% / 1%)"},
		{kind: "type", name: "number", value: "round(down, 1px, 2px)"},
		{kind: "type", name: "number", value: "mod(1px, 2px)"},
		{kind: "type", name: "number", value: "sign(-1px)", matched: true},
		{kind: "type", name: "number-percentage", value: "calc(1% / 1%)", matched: true},
		{kind: "type", name: "integer", value: "calc(1.5)", matched: true},
		{kind: "type", name: "integer", value: "calc(1px)"},
		{kind: "type", name: "angle", value: "calc(1deg + 1grad)", matched: true},
		{kind: "type", name: "angle", value: "calc(1deg + 0)"},
		{kind: "type", name: "angle-percentage", value: "calc(1deg + 10%)", matched: true},
		{kind: "type", name: "time", value: "calc(1s - 1ms)", matched: true},
		{kind: "type", name: "time-percentage", value: "calc(1s - 10%)", matched: true},
		{kind: "type", name: "frequency", value: "calc(1hz + 1khz)", matched: true},
		{kind: "type", name: "frequency", value: "calc(1 / 1s)"},
		{kind: "type", name: "frequency-percentage", value: "calc(1khz * 10%)"},
		{kind: "type", name: "resolution", value: "calc(1dppx * 2)", matched: true},
		{kind: "type", name: "resolution", value: "calc(1x + 1dpi)", matched: true},
		{kind: "type", name: "dimension", value: "calc(1px)", matched: true},
		{kind: "type", name: "dimension", value: "calc(1deg * 2)", matched: true},
		{kind: "type", name: "dimension", value: "calc(1)"},
		{kind: "type", name: "dimension", value: "calc(1%)"},
		{kind: "type", name: "dimension", value: "calc(1foo)"},
		// <flex>, <decibel> and <semitones> are not math function types
		{kind: "type", name: "flex", value: "calc(1fr)"},
		{kind: "type", name: "flex", value: "calc(2 * 1fr)"},
		{kind: "type", name: "decibel", value: "calc(2 * 1db)"},
		{kind: "type", name: "semitones", value: "calc(2 * 1st)"},
		// <zero> takes a literal 0 only
		{kind: "type", name: "zero", value: "calc(0)"},
		// non-numeric types still do not take math functions
		{kind: "type", name: "custom-ident", value: "calc(1)"},
		{kind: "type", name: "any-value", value: "calc(red)", matched: true},
		// an unclosed function at the end of the value is closed, as in
		// CSS Syntax 3 §5.4.9
		{kind: "type", name: "length", value: "calc(1px + (2px", matched: true},
		{kind: "type", name: "length", value: "calc(1px +"},
	}

	runMatchCases(t, DefaultLexer(), cases)
}

func TestSpecMathMatchSyntax(t *testing.T) {
	cases := []matchCase{
		// a <percentage> alternative of a numeric type lets percentages
		// mix with that type, as in <length-percentage>
		{kind: "syntax", name: "<length> | <percentage>", value: "calc(1px + 10%)", matched: true},
		{kind: "syntax", name: "<percentage> | <length>", value: "calc(1px + 10%)", matched: true},
		{kind: "syntax", name: "<length> | <percentage> | auto", value: "calc(1px + 10%)", matched: true},
		{kind: "syntax", name: "[ <length> | <percentage> ]{2}", value: "calc(1px + 10%) calc(10% - 1em)", matched: true},
		{kind: "syntax", name: "<length> <percentage>", value: "calc(1px + 10%) 1%"},
		{kind: "syntax", name: "<length> || <percentage>", value: "calc(1px + 10%)"},
		{kind: "syntax", name: "<length>", value: "calc(1px + 10%)"},
		{kind: "syntax", name: "<time> | <percentage>", value: "calc(1s + 10%)", matched: true},
		{kind: "syntax", name: "<angle> | <percentage>", value: "calc(1deg + 10%)", matched: true},
		{kind: "syntax", name: "<frequency> | <percentage>", value: "calc(1hz + 10%)", matched: true},
		{kind: "syntax", name: "<resolution> | <percentage>", value: "calc(1dppx + 10%)"},
		// §10.12 ranges are not checked for math functions
		{kind: "syntax", name: "<length [0,∞]>", value: "calc(-1px)", matched: true},
		{kind: "syntax", name: "<integer [1,∞]>", value: "calc(0)", matched: true},
		{kind: "syntax", name: "<number [0,1]>", value: "calc(2)", matched: true},
	}

	runMatchCases(t, DefaultLexer(), cases)
}

// TestSpecMathTrace checks that a math function is matched as one token
// run by the type it resolves to.
func TestSpecMathTrace(t *testing.T) {
	lexer := DefaultLexer()
	node, err := Parse("calc(1px + 10%) 2px", ParseOptions{Context: "value"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if _, err := lexer.MatchProperty("margin", node); err != nil {
		t.Fatalf("MatchProperty(margin) error = %v", err)
	}
	fragments, err := lexer.FindValueFragments("margin", node, SyntaxRef{Kind: KindType, Name: "length"})
	if err != nil {
		t.Fatalf("FindValueFragments() error = %v", err)
	}
	if len(fragments) != 2 {
		t.Fatalf("FindValueFragments(margin, <length>) = %d fragments, want 2", len(fragments))
	}
}

// TestSpecMathIterations checks that type checking keeps the matcher from
// iterating more for nested math functions.
func TestSpecMathIterations(t *testing.T) {
	deep := strings.Repeat("calc(", 30) + "1px" + strings.Repeat(")", 30)
	cases := []struct {
		property string
		value    string
		want     bool
	}{
		{"width", deep, true},
		{"width", "calc(" + strings.Repeat("(", 200) + "1px" + strings.Repeat(")", 200) + ")", true},
		{"width", "calc(" + strings.Repeat("1px + ", 500) + "1px)", true},
		{"width", "min(" + strings.Repeat("calc(1px + 2%), ", 100) + "1px)", true},
		{"grid-template-columns", strings.Repeat("calc(10% + 1px) ", 20), true},
		{"transition-duration", strings.Repeat("calc(1s * 2), ", 50) + "1s", true},
		{"margin", "calc(1px + 1s) calc(1px + 1s) calc(1px + 1s) calc(1px + 1s)", false},
	}

	lexer := DefaultLexer()
	for _, tc := range cases {
		result, err := lexer.MatchPropertyString(tc.property, tc.value)
		if got := err == nil; got != tc.want {
			t.Errorf("MatchPropertyString(%q, %.40q...) matched = %t, want %t (error: %v)", tc.property, tc.value, got, tc.want, err)
		}
		if result.Iterations >= iterationLimit {
			t.Errorf("MatchPropertyString(%q, %.40q...) iterations = %d, want below %d", tc.property, tc.value, result.Iterations, iterationLimit)
		}
	}
}

// TestSpecMathNestingLimit checks that a math function nested deeper than
// the supported depth is invalid rather than exhausting the stack (§10.8:
// a calculation with more nesting than supported is invalid).
func TestSpecMathNestingLimit(t *testing.T) {
	lexer := DefaultLexer()
	n := maxCalcDepth + 1
	for _, value := range []string{
		strings.Repeat("calc(", n) + "1px" + strings.Repeat(")", n),
		"calc(" + strings.Repeat("(", n) + "1px" + strings.Repeat(")", n) + ")",
	} {
		if _, err := lexer.MatchPropertyString("width", value); err == nil {
			t.Errorf("MatchPropertyString(width, %d levels of nesting) matched, want a mismatch", n)
		}
	}
}

// specMathFixtureInvalid lists the values of upstream's
// definition-syntax-match fixtures that are valid upstream but invalid by
// Values 4 §10.9, by fixture key.
var specMathFixtureInvalid = map[string][]string{
	// percentages are not allowed in a <length> that is not a
	// <length-percentage>
	"<length> || <size>": {"calc(1px + (2px / 3) + 10%) normal"},
	// <flex> is not a math function type (CSS Grid 2 §7.2.4), nor are the
	// CSS Speech <decibel> and <semitones>
	"<flex>":      {"calc(2 * 1fr)"},
	"<decibel>":   {"calc(2 * 1db)"},
	"<semitones>": {"calc(2 * 1st)"},
	"math-length": {
		// percentages are not allowed in <length>
		"calc(1px + 1%)", "CALC(1px + 1%)",
		"-moz-calc(1px + 1%)", "-MOZ-CALC(1px + 1%)",
		"-webkit-calc(1px + 1%)", "-WEBKIT-CALC(1px + 1%)",
		// these resolve to <angle>
		"asin(0.5)", "acos(0.5)", "atan(1)", "atan2(1px, 2px)",
	},
	// these resolve to <length>
	"math-number": {"round(down, 1px, 2px)", "mod(1px, 2px)", "rem(1px, 2px)"},
}

// specMathFixtureValues moves the values specMathFixtureInvalid lists for
// the fixture key from valid to invalid.
func specMathFixtureValues(key string, valid, invalid []string) ([]string, []string) {
	moved := specMathFixtureInvalid[key]
	if len(moved) == 0 {
		return valid, invalid
	}

	var stillValid []string
	invalid = slices.Clone(invalid)
	for _, value := range valid {
		if slices.Contains(moved, value) {
			invalid = append(invalid, value)
			continue
		}
		stillValid = append(stillValid, value)
	}
	return stillValid, invalid
}

// TestSpecMathFixtureOverrides checks that every value
// specMathFixtureInvalid moves is a valid value of its fixture, so that
// the list does not go stale.
func TestSpecMathFixtureOverrides(t *testing.T) {
	seen := make(map[string]map[string]bool)
	forEachDefinitionSyntaxMatchTest(t, func(_ *testing.T, key string, test definitionSyntaxMatchTest) {
		if _, ok := specMathFixtureInvalid[key]; !ok {
			return
		}
		if seen[key] == nil {
			seen[key] = make(map[string]bool)
		}
		for _, value := range test.Valid {
			seen[key][value] = true
		}
	})

	for key, values := range specMathFixtureInvalid {
		for _, value := range values {
			if !seen[key][value] {
				t.Errorf("specMathFixtureInvalid[%q] has %q, which is not a valid value of that fixture", key, value)
			}
		}
	}
}
