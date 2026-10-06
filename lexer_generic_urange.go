// Ported from lib/lexer/generic-urange.js.

package csstree

import (
	"github.com/typelate/csstree/definitionsyntax"
	"github.com/typelate/csstree/tokenizer"
)

const (
	urangePlusSign     = 0x002B // U+002B PLUS SIGN (+)
	urangeHyphenMinus  = 0x002D // U+002D HYPHEN-MINUS (-)
	urangeQuestionMark = 0x003F // U+003F QUESTION MARK (?)
	urangeU            = 0x0075 // U+0075 LATIN SMALL LETTER U (u)
)

func tokenStartsWith(token *matchToken, code int) bool {
	return tokenizer.CharCodeAt(token.value, 0) == code
}

func hexSequence(token *matchToken, offset int, allowDash bool) int {
	hexlen := 0

	for pos := offset; pos < len(token.value); pos++ {
		code := tokenizer.CharCodeAt(token.value, pos)

		if code == urangeHyphenMinus && allowDash && hexlen != 0 {
			hexSequence(token, offset+hexlen+1, false)
			return 6 // dissallow following question marks
		}

		if !tokenizer.IsHexDigit(code) {
			return 0 // not a hex digit
		}

		hexlen++
		if hexlen > 6 {
			return 0 // too many hex digits
		}
	}

	return hexlen
}

func withQuestionMarkSequence(consumed, length int, getNextToken func(offset int) *matchToken) int {
	if consumed == 0 {
		return 0 // nothing consumed
	}

	for isDelimToken(getNextToken(length), urangeQuestionMark) {
		consumed++
		if consumed > 6 {
			return 0 // too many question marks
		}

		length++
	}

	return length
}

// https://drafts.csswg.org/css-syntax/#urange
// Informally, the <urange> production has three forms:
// U+0001
//
//	Defines a range consisting of a single code point, in this case the code point "1".
//
// U+0001-00ff
//
//	Defines a range of codepoints between the first and the second value, in this case
//	the range between "1" and "ff" (255 in decimal) inclusive.
//
// U+00??
//
//	Defines a range of codepoints where the "?" characters range over all hex digits,
//	in this case defining the same as the value U+0000-00ff.
//
// In each form, a maximum of 6 digits is allowed for each hexadecimal number (if you treat "?" as a hexadecimal digit).
//
// <urange> =
//
//	u '+' <ident-token> '?'* |
//	u <dimension-token> '?'* |
//	u <number-token> '?'* |
//	u <number-token> <dimension-token> |
//	u <number-token> <number-token> |
//	u '+' '?'+
func urange(token *matchToken, getNextToken func(offset int) *matchToken, _ *definitionsyntax.Range) int {
	length := 0

	// should start with `u` or `U`
	if token == nil || token.typ != tokenizer.Ident || !tokenizer.CmpChar(token.value, 0, urangeU) {
		return 0
	}

	length++
	token = getNextToken(length)
	if token == nil {
		return 0
	}

	// u '+' <ident-token> '?'*
	// u '+' '?'+
	if isDelimToken(token, urangePlusSign) {
		length++
		token = getNextToken(length)
		if token == nil {
			return 0
		}

		if token.typ == tokenizer.Ident {
			// u '+' <ident-token> '?'*
			length++
			return withQuestionMarkSequence(hexSequence(token, 0, true), length, getNextToken)
		}

		if isDelimToken(token, urangeQuestionMark) {
			// u '+' '?'+
			length++
			return withQuestionMarkSequence(1, length, getNextToken)
		}

		// Hex digit or question mark is expected
		return 0
	}

	// u <number-token> '?'*
	// u <number-token> <dimension-token>
	// u <number-token> <number-token>
	if token.typ == tokenizer.Number {
		consumedHexLength := hexSequence(token, 1, true)
		if consumedHexLength == 0 {
			return 0
		}

		length++
		token = getNextToken(length)
		if token == nil {
			// u <number-token> <eof>
			return length
		}

		if token.typ == tokenizer.Dimension || token.typ == tokenizer.Number {
			// u <number-token> <dimension-token>
			// u <number-token> <number-token>
			if !tokenStartsWith(token, urangeHyphenMinus) || hexSequence(token, 1, false) == 0 {
				return 0
			}

			return length + 1
		}

		// u <number-token> '?'*
		return withQuestionMarkSequence(consumedHexLength, length, getNextToken)
	}

	// u <dimension-token> '?'*
	if token.typ == tokenizer.Dimension {
		length++
		return withQuestionMarkSequence(hexSequence(token, 1, true), length, getNextToken)
	}

	return 0
}
