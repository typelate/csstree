// Ported from lib/lexer/generic.js.

package csstree

import (
	"math"
	"slices"

	"github.com/typelate/csstree/definitionsyntax"
	"github.com/typelate/csstree/internal/jsnum"
	"github.com/typelate/csstree/tokenizer"
)

// genericFunc is a generic type: it returns how many of tokens, the rest of
// the input, it consumes from their start, 0 for a mismatch. bounds is the
// range restriction of the type reference being matched, nil when there is
// none.
type genericFunc func(tokens []matchToken, bounds *definitionsyntax.Range) int

// genericType is a generic type with its name; createGenericTypes returns
// them in upstream's object key order. calc says how the matcher type
// checks a math function for the type; the zero value leaves math
// functions to fn.
type genericType struct {
	name string
	fn   genericFunc
	calc calcTarget
}

// tokenAt returns tokens[i], nil past the end.
func tokenAt(tokens []matchToken, i int) *matchToken {
	if i < len(tokens) {
		return &tokens[i]
	}
	return nil
}

// balancePair returns the closing token type for an opening one (upstream
// the balancePair Map), 0 for other types.
func balancePair(tokenType tokenizer.TokenType) tokenizer.TokenType {
	switch tokenType {
	case tokenizer.Function, tokenizer.LeftParenthesis:
		return tokenizer.RightParenthesis
	case tokenizer.LeftSquareBracket:
		return tokenizer.RightSquareBracket
	case tokenizer.LeftCurlyBracket:
		return tokenizer.RightCurlyBracket
	}
	return 0
}

// IE postfix hack, i.e. 123\0 or 123px\9
func isPostfixIeHack(str string, offset int) bool {
	if offset != len(str)-2 {
		return false
	}

	return tokenizer.CharCodeAt(str, offset) == '\\' &&
		tokenizer.IsDigit(tokenizer.CharCodeAt(str, offset+1))
}

// outOfRange reports whether the number at the start of value is outside
// bounds. numEnd < 0 is upstream's omitted numEnd.
func outOfRange(bounds *definitionsyntax.Range, value string, numEnd int) bool {
	return outOfRangeUnit(bounds, value, numEnd, "")
}

// outOfRangeUnit reports whether the number at the start of value, of the
// lowercase unit ("" for a number), is outside bounds. Upstream skips the
// bounds with a unit (a FIXME: a type conversion is not implemented);
// compareToBound compares them as CSS Values 4 §5.1 says.
func outOfRangeUnit(bounds *definitionsyntax.Range, value string, numEnd int, unit string) bool {
	if bounds != nil {
		numStr := value
		if numEnd >= 0 && numEnd != len(value) {
			numStr = value[:numEnd]
		}
		num := jsnum.Parse(numStr)

		if math.IsNaN(num) {
			return true
		}

		if bounds.Min != nil {
			if order, ok := compareToBound(num, unit, bounds.Min); ok && order < 0 {
				return true
			}
		}

		if bounds.Max != nil {
			if order, ok := compareToBound(num, unit, bounds.Max); ok && order > 0 {
				return true
			}
		}
	}

	return false
}

// scanBalanced returns how many tokens from the start of tokens form a
// balanced run: it stops before an unmatched ), ] or }, and before a token
// for which stop, when not nil, returns true; topLevel tells whether the
// token is outside every function and block.
func scanBalanced(tokens []matchToken, stop func(token *matchToken, topLevel bool) bool) int {
	// the closing token types of the open functions and blocks
	var closers []tokenizer.TokenType

	for i := range tokens {
		token := &tokens[i]
		if stop != nil && stop(token, len(closers) == 0) {
			return i
		}

		switch token.typ {
		case tokenizer.RightCurlyBracket, tokenizer.RightParenthesis, tokenizer.RightSquareBracket:
			if len(closers) == 0 || token.typ != closers[len(closers)-1] {
				return i
			}
			closers = closers[:len(closers)-1]

		case tokenizer.Function, tokenizer.LeftParenthesis, tokenizer.LeftSquareBracket, tokenizer.LeftCurlyBracket:
			closers = append(closers, balancePair(token.typ))
		}
	}

	return len(tokens)
}

// consumeFunction returns how many of tokens, which start with a function
// token or an opening bracket, the function or block takes: up to its
// closing token, or all of them when it is not closed.
func consumeFunction(tokens []matchToken) int {
	if len(tokens) == 0 {
		return 0
	}

	length := 1 + scanBalanced(tokens[1:], nil)
	if length < len(tokens) && tokens[length].typ == balancePair(tokens[0].typ) {
		length++
	}
	return length
}

func tokenType(expectedTokenType tokenizer.TokenType) genericFunc {
	return func(tokens []matchToken, _ *definitionsyntax.Range) int {
		if len(tokens) == 0 || tokens[0].typ != expectedTokenType {
			return 0
		}

		return 1
	}
}

// =========================
// Complex types
//

// https://drafts.csswg.org/css-values-4/#custom-idents
// 4.2. Author-defined Identifiers: the <custom-ident> type
// Some properties accept arbitrary author-defined identifiers as a component value.
// This generic data type is denoted by <custom-ident>, and represents any valid CSS identifier
// that would not be misinterpreted as a pre-defined keyword in that property’s value definition.
//
// See also: https://developer.mozilla.org/en-US/docs/Web/CSS/custom-ident
func customIdent(tokens []matchToken, _ *definitionsyntax.Range) int {
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Ident {
		return 0
	}
	token := &tokens[0]

	// the identifier's value: escapes decoded, compared ASCII
	// case-insensitively
	name := tokenName(token)

	// The CSS-wide keywords are not valid <custom-ident>s
	if isCSSWideKeyword(name) {
		return 0
	}

	// The default keyword is reserved and is also not a valid <custom-ident>
	if name == "default" {
		return 0
	}

	// TODO: ignore property specific keywords (as described https://developer.mozilla.org/en-US/docs/Web/CSS/custom-ident)
	// Specifications using <custom-ident> must specify clearly what other keywords
	// are excluded from <custom-ident>, if any—for example by saying that any pre-defined keywords
	// in that property’s value definition are excluded. Excluded keywords are excluded
	// in all ASCII case permutations.

	return 1
}

// https://drafts.csswg.org/css-values-4/#dashed-idents
// The <dashed-ident> production is a <custom-ident>, with all the case-sensitivity that implies,
// with the additional restriction that it must start with two dashes (U+002D HYPHEN-MINUS).
func dashedIdent(tokens []matchToken, _ *definitionsyntax.Range) int {
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Ident {
		return 0
	}
	token := &tokens[0]

	// ... must start with two dashes (U+002D HYPHEN-MINUS), in its value
	// (escapes decoded)
	value := tokenValue(token)
	if tokenizer.CharCodeAt(value, 0) != '-' || tokenizer.CharCodeAt(value, 1) != '-' {
		return 0
	}

	return 1
}

// https://drafts.csswg.org/css-variables/#typedef-custom-property-name
// A custom property is any property whose name starts with two dashes (U+002D HYPHEN-MINUS), like --foo.
// The <custom-property-name> production corresponds to this: it’s defined as any <dashed-ident>
// (a valid identifier that starts with two dashes), except -- itself, which is reserved for future use by CSS.
func customPropertyName(tokens []matchToken, bounds *definitionsyntax.Range) int {
	// ... it’s defined as any <dashed-ident>
	if dashedIdent(tokens, bounds) == 0 {
		return 0
	}

	// ... except -- itself, which is reserved for future use by CSS
	if tokenValue(&tokens[0]) == "--" {
		return 0
	}

	return 1
}

// https://drafts.csswg.org/css-color-4/#hex-notation
// The syntax of a <hex-color> is a <hash-token> token whose value consists of 3, 4, 6, or 8 hexadecimal digits.
// In other words, a hex color is written as a hash character, "#", followed by some number of digits 0-9 or
// letters a-f (the case of the letters doesn’t matter - #00ff00 is identical to #00FF00).
func hexColor(tokens []matchToken, _ *definitionsyntax.Range) int {
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Hash {
		return 0
	}
	token := &tokens[0]

	length := len(token.value)

	// valid values (length): #rgb (4), #rgba (5), #rrggbb (7), #rrggbbaa (9)
	if length != 4 && length != 5 && length != 7 && length != 9 {
		return 0
	}

	for i := 1; i < length; i++ {
		if !tokenizer.IsHexDigit(tokenizer.CharCodeAt(token.value, i)) {
			return 0
		}
	}

	return 1
}

func idSelector(tokens []matchToken, _ *definitionsyntax.Range) int {
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Hash {
		return 0
	}
	token := &tokens[0]

	if !tokenizer.IsIdentifierStart(tokenizer.CharCodeAt(token.value, 1), tokenizer.CharCodeAt(token.value, 2), tokenizer.CharCodeAt(token.value, 3)) {
		return 0
	}

	return 1
}

// https://drafts.csswg.org/css-syntax/#any-value
// It represents the entirety of what a valid declaration can have as its value.
func declarationValue(tokens []matchToken, _ *definitionsyntax.Range) int {
	// The <declaration-value> production matches any sequence of one or more tokens,
	// so long as the sequence does not contain ...
	return scanBalanced(tokens, func(token *matchToken, topLevel bool) bool {
		switch token.typ {
		// ... <bad-string-token>, <bad-url-token>,
		case tokenizer.BadString, tokenizer.BadURL:
			return true
		// ... unmatched <)-token>, <]-token>, or <}-token> (scanBalanced
		// stops before them),
		// ... or top-level <semicolon-token> tokens
		case tokenizer.Semicolon:
			return topLevel
		// ... or <delim-token> tokens with a value of "!"
		case tokenizer.Delim:
			return topLevel && token.value == "!"
		}
		return false
	})
}

// https://drafts.csswg.org/css-syntax/#any-value
// The <any-value> production is identical to <declaration-value>, but also
// allows top-level <semicolon-token> tokens and <delim-token> tokens
// with a value of "!". It represents the entirety of what valid CSS can be in any context.
func anyValue(tokens []matchToken, _ *definitionsyntax.Range) int {
	// The <any-value> production matches any sequence of one or more tokens,
	// so long as the sequence does not contain <bad-string-token>,
	// <bad-url-token>, unmatched <)-token>, <]-token>, or <}-token>
	return scanBalanced(tokens, func(token *matchToken, _ bool) bool {
		return token.typ == tokenizer.BadString || token.typ == tokenizer.BadURL
	})
}

// =========================
// Dimensions
//

// dimension returns a generic type for dimension tokens whose unit is in
// units.
func dimension(units []string) genericFunc {
	unitSet := make(map[string]bool, len(units))
	for _, unit := range units {
		unitSet[unit] = true
	}

	return func(tokens []matchToken, bounds *definitionsyntax.Range) int {
		unit, ok := dimensionTokenUnit(tokens, bounds)
		if !ok || !unitSet[unit] {
			return 0
		}
		return 1
	}
}

// anyDimension is the generic type of dimension tokens of any unit
// (upstream dimension(null)).
func anyDimension(tokens []matchToken, bounds *definitionsyntax.Range) int {
	if _, ok := dimensionTokenUnit(tokens, bounds); !ok {
		return 0
	}
	return 1
}

// dimensionTokenUnit returns the lowercase unit of a dimension token in
// the range bounds, and false for other tokens and dimensions out of the
// range.
func dimensionTokenUnit(tokens []matchToken, bounds *definitionsyntax.Range) (string, bool) {
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Dimension {
		return "", false
	}
	token := &tokens[0]

	numberEnd := tokenizer.ConsumeNumber(token.value, 0)

	// the unit without the IE postfix hack, i.e. 123px\0 or 123px\9,
	// and with escapes decoded, compared ASCII case-insensitively
	unit := asciiLowerCase(dimensionUnit(token.value, numberEnd))

	// check range if specified
	if outOfRangeUnit(bounds, token.value, numberEnd, unit) {
		return "", false
	}

	return unit, true
}

// =========================
// Percentage
//

// §5.5. Percentages: the <percentage> type
// https://drafts.csswg.org/css-values-4/#percentages
func percentage(tokens []matchToken, bounds *definitionsyntax.Range) int {
	// ... corresponds to the <percentage-token> production
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Percentage {
		return 0
	}
	token := &tokens[0]

	// check range if specified
	if outOfRangeUnit(bounds, token.value, len(token.value)-1, "%") {
		return 0
	}

	return 1
}

// =========================
// Numeric
//

// https://drafts.csswg.org/css-values-4/#numbers
// The value <zero> represents a literal number with the value 0. Expressions that merely
// evaluate to a <number> with the value 0 (for example, calc(0)) do not match <zero>;
// only literal <number-token>s do.
func zero(tokens []matchToken, bounds *definitionsyntax.Range) int {
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Number {
		return 0
	}
	token := &tokens[0]

	// a zero is out of a range such as [1px,∞]
	if jsnum.Parse(token.value) != 0 || outOfRange(bounds, "0", -1) {
		return 0
	}

	return 1
}

// zeroOr returns a generic type that takes a <zero> or what next takes.
func zeroOr(next genericFunc) genericFunc {
	return func(tokens []matchToken, bounds *definitionsyntax.Range) int {
		if zero(tokens, bounds) == 1 {
			return 1
		}

		return next(tokens, bounds)
	}
}

// § 5.3. Real Numbers: the <number> type
// https://drafts.csswg.org/css-values-4/#numbers
// Number values are denoted by <number>, and represent real numbers, possibly with a fractional component.
// ... It corresponds to the <number-token> production
func number(tokens []matchToken, bounds *definitionsyntax.Range) int {
	// ... a <number-token>, or the <dimension-token> of a number with the
	// IE postfix hack (123\9); upstream reads any token whose text is a
	// number, such as the delims + and - or the ident e5
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Number && tokens[0].typ != tokenizer.Dimension {
		return 0
	}
	token := &tokens[0]

	numberEnd := tokenizer.ConsumeNumber(token.value, 0)
	isNumber := token.typ == tokenizer.Number && numberEnd == len(token.value)
	if !isNumber && (token.typ != tokenizer.Dimension || !isPostfixIeHack(token.value, numberEnd)) {
		return 0
	}

	// check range if specified
	if outOfRange(bounds, token.value, numberEnd) {
		return 0
	}

	return 1
}

// §5.2. Integers: the <integer> type
// https://drafts.csswg.org/css-values-4/#integers
func integer(tokens []matchToken, bounds *definitionsyntax.Range) int {
	// ... corresponds to a subset of the <number-token> production
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Number {
		return 0
	}
	token := &tokens[0]

	// The first digit of an integer may be immediately preceded by `-` or `+` to indicate the integer’s sign.
	i := 0
	if isSign(tokenizer.CharCodeAt(token.value, 0)) {
		i = 1
	}

	// When written literally, an integer is one or more decimal digits 0 through 9 ...
	for ; i < len(token.value); i++ {
		if !tokenizer.IsDigit(tokenizer.CharCodeAt(token.value, i)) {
			return 0
		}
	}

	// check range if specified
	if outOfRange(bounds, token.value, i) {
		return 0
	}

	return 1
}

// token types
func tokenTypes() []genericType {
	return []genericType{
		{name: "ident-token", fn: tokenType(tokenizer.Ident)},
		{name: "function-token", fn: tokenType(tokenizer.Function)},
		{name: "at-keyword-token", fn: tokenType(tokenizer.AtKeyword)},
		{name: "hash-token", fn: tokenType(tokenizer.Hash)},
		{name: "string-token", fn: tokenType(tokenizer.String)},
		{name: "bad-string-token", fn: tokenType(tokenizer.BadString)},
		{name: "url-token", fn: tokenType(tokenizer.URL)},
		{name: "bad-url-token", fn: tokenType(tokenizer.BadURL)},
		{name: "delim-token", fn: tokenType(tokenizer.Delim)},
		{name: "number-token", fn: tokenType(tokenizer.Number)},
		{name: "percentage-token", fn: tokenType(tokenizer.Percentage)},
		{name: "dimension-token", fn: tokenType(tokenizer.Dimension)},
		{name: "whitespace-token", fn: tokenType(tokenizer.WhiteSpace)},
		{name: "CDO-token", fn: tokenType(tokenizer.CDO)},
		{name: "CDC-token", fn: tokenType(tokenizer.CDC)},
		{name: "colon-token", fn: tokenType(tokenizer.Colon)},
		{name: "semicolon-token", fn: tokenType(tokenizer.Semicolon)},
		{name: "comma-token", fn: tokenType(tokenizer.Comma)},
		{name: "[-token", fn: tokenType(tokenizer.LeftSquareBracket)},
		{name: "]-token", fn: tokenType(tokenizer.RightSquareBracket)},
		{name: "(-token", fn: tokenType(tokenizer.LeftParenthesis)},
		{name: ")-token", fn: tokenType(tokenizer.RightParenthesis)},
		{name: "{-token", fn: tokenType(tokenizer.LeftCurlyBracket)},
		{name: "}-token", fn: tokenType(tokenizer.RightCurlyBracket)},
	}
}

// productionTypes returns the token production types, whose numeric types
// type check math functions with the base types of units.
func productionTypes(units calcUnits) []genericType {
	return []genericType{
		// token type aliases
		{name: "string", fn: tokenType(tokenizer.String)},
		{name: "ident", fn: tokenType(tokenizer.Ident)},

		// percentage
		{name: "percentage", fn: percentage, calc: percentageCalcTarget(units)},

		// numeric
		{name: "zero", fn: zero},
		{name: "number", fn: number, calc: numberCalcTarget(units)},
		{name: "integer", fn: integer, calc: numberCalcTarget(units)},

		// complex types
		{name: "custom-ident", fn: customIdent},
		{name: "dashed-ident", fn: dashedIdent},
		{name: "custom-property-name", fn: customPropertyName},
		{name: "hex-color", fn: hexColor},
		{name: "id-selector", fn: idSelector}, // element( <id-selector> )
		{name: "an-plus-b", fn: anPlusB},
		{name: "urange", fn: urange},
		{name: "declaration-value", fn: declarationValue},
		{name: "any-value", fn: anyValue},
	}
}

// createDimensionTypes returns the dimension types, which depend on the
// units of each unit group.
func createDimensionTypes(units map[string][]string, bases calcUnits) []genericType {
	types := []genericType{
		{name: "dimension", fn: anyDimension, calc: dimensionCalcTarget(bases)},
	}

	// in upstream's key order
	for _, group := range []unitGroup{angleUnits, decibelUnits, frequencyUnits, flexUnits, lengthUnits, resolutionUnits, semitonesUnits, timeUnits} {
		name := group.name()
		fn := dimension(units[name])
		if group == lengthUnits {
			// Values 4 §6: a unitless zero is a <length>
			fn = zeroOr(fn)
		}
		types = append(types, genericType{name: name, fn: fn, calc: unitGroupCalcTarget(group, bases)})
	}

	return types
}

// The <attr-unit> production matches any identifier that is an ASCII case-insensitive
// match for the name of a CSS dimension unit, such as px, or the <delim-token> %.
func createAttrUnit(units map[string][]string) genericFunc {
	unitSet := make(map[string]bool)

	for group := range unitGroupCount {
		for _, unit := range units[group.name()] {
			unitSet[toLowerCase(unit)] = true
		}
	}

	return func(tokens []matchToken, _ *definitionsyntax.Range) int {
		if len(tokens) == 0 {
			return 0
		}
		token := &tokens[0]

		if token.typ == tokenizer.Delim && token.value == "%" {
			return 1
		}

		if token.typ == tokenizer.Ident && unitSet[asciiLowerCase(tokenValue(token))] {
			return 1
		}

		return 0
	}
}

func createGenericTypes(units map[string][]string) []genericType {
	bases := newCalcUnits(units)
	return slices.Concat(
		tokenTypes(),
		productionTypes(bases),
		createDimensionTypes(units, bases),
		[]genericType{{name: "attr-unit", fn: createAttrUnit(units)}},
	)
}
