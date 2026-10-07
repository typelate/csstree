// Ported from lib/lexer/generic-urange.js.

package csstree

import (
	"strings"
	"unicode"

	"github.com/typelate/csstree/definitionsyntax"
	"github.com/typelate/csstree/tokenizer"
)

// maxURangeDigits is the most hex digits a number of a <urange> may have,
// counting question marks as hex digits.
const maxURangeDigits = 6

func tokenStartsWith(token *matchToken, c rune) bool {
	return tokenizer.CharCodeAt(token.value, 0) == int(c)
}

// hexSequence returns how many hex digits the value of token has from
// offset, 0 when there are other characters or too many. With allowDash, a
// - after the digits starts the end of a range: hexSequence then returns
// maxURangeDigits, so that no question mark may follow.
func hexSequence(token *matchToken, offset int, allowDash bool) int {
	hexlen := 0

	for pos := offset; pos < len(token.value); pos++ {
		code := tokenizer.CharCodeAt(token.value, pos)

		if code == '-' && allowDash && hexlen != 0 {
			// upstream checks the digits after the - here, and ignores
			// the result; validURange checks them
			return maxURangeDigits // disallow following question marks
		}

		if !tokenizer.IsHexDigit(code) {
			return 0 // not a hex digit
		}

		hexlen++
		if hexlen > maxURangeDigits {
			return 0 // too many hex digits
		}
	}

	return hexlen
}

// withQuestionMarkSequence returns length plus the question marks at
// index length of tokens, 0 when consumed, the hex digits before them, is
// 0 or the digits and question marks are too many.
func withQuestionMarkSequence(consumed, length int, tokens []matchToken) int {
	if consumed == 0 {
		return 0 // nothing consumed
	}

	for isDelimToken(tokenAt(tokens, length), '?') {
		consumed++
		if consumed > maxURangeDigits {
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
//
// The tokens must then interpret into a valid range, see validURange.
func urange(tokens []matchToken, _ *definitionsyntax.Range) int {
	length := urangeTokens(tokens)
	if length == 0 {
		return 0
	}

	// Skipping the first u token, concatenate the representations of all
	// the tokens in the production together.
	var text strings.Builder
	for i := 1; i < length; i++ {
		text.WriteString(tokens[i].value)
	}

	if !validURange(text.String()) {
		return 0
	}

	return length
}

// urangeTokens returns how many tokens from the start of tokens have the
// forms of <urange>, 0 for none.
func urangeTokens(tokens []matchToken) int {
	length := 0
	token := tokenAt(tokens, length)

	// should start with the ident u (upstream: an ident starting with
	// `u` or `U`, so ua+1 was a <urange>)
	if token == nil || token.typ != tokenizer.Ident || !asciiEqualFold(tokenValue(token), "u") {
		return 0
	}

	length++
	token = tokenAt(tokens, length)
	if token == nil {
		return 0
	}

	// u '+' <ident-token> '?'*
	// u '+' '?'+
	if isDelimToken(token, '+') {
		length++
		token = tokenAt(tokens, length)
		if token == nil {
			return 0
		}

		if token.typ == tokenizer.Ident {
			// u '+' <ident-token> '?'*
			length++
			return withQuestionMarkSequence(hexSequence(token, 0, true), length, tokens)
		}

		if isDelimToken(token, '?') {
			// u '+' '?'+
			length++
			return withQuestionMarkSequence(1, length, tokens)
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
		token = tokenAt(tokens, length)
		if token == nil {
			// u <number-token> <eof>
			return length
		}

		if token.typ == tokenizer.Dimension || token.typ == tokenizer.Number {
			// u <number-token> <dimension-token>
			// u <number-token> <number-token>
			if !tokenStartsWith(token, '-') || hexSequence(token, 1, false) == 0 {
				return 0
			}

			return length + 1
		}

		// u <number-token> '?'*
		return withQuestionMarkSequence(consumedHexLength, length, tokens)
	}

	// u <dimension-token> '?'*
	if token.typ == tokenizer.Dimension {
		length++
		return withQuestionMarkSequence(hexSequence(token, 1, true), length, tokens)
	}

	return 0
}

// validURange reports whether text, the representations of the tokens of
// a <urange> after its u, is a valid range: it interprets text into a
// start and an end value as CSS Syntax 3 §7.1 says (Candidate
// Recommendation Draft, 24 December 2021), and the range is invalid when
// the end is greater than the maximum allowed code point or the start is
// greater than the end.
func validURange(text string) bool {
	// If the first character of text is U+002B PLUS SIGN, consume it.
	// Otherwise, this is an invalid <urange>.
	if !strings.HasPrefix(text, "+") {
		return false
	}
	text = text[1:]

	// Consume as many hex digits from text as possible, then consume as
	// many U+003F QUESTION MARK (?) code points as possible. If zero code
	// points were consumed, or more than six code points were consumed,
	// this is an invalid <urange>.
	digits := hexDigitsPrefix(text)
	questionMarks := len(text[digits:]) - len(strings.TrimLeft(text[digits:], "?"))
	consumed := text[:digits+questionMarks]
	text = text[len(consumed):]
	if len(consumed) == 0 || len(consumed) > maxURangeDigits {
		return false
	}

	var start, end int64
	switch {
	case questionMarks > 0:
		// If there are any code points left in text, this is an invalid
		// <urange>. The start value is the consumed code points with the ?
		// replaced by 0, the end value with the ? replaced by F.
		if text != "" {
			return false
		}
		start = parseHex(strings.ReplaceAll(consumed, "?", "0"))
		end = parseHex(strings.ReplaceAll(consumed, "?", "F"))

	case text == "":
		// If there are no code points left in text, the end value is the
		// same as the start value.
		start = parseHex(consumed)
		end = start

	default:
		start = parseHex(consumed)

		// If the next code point in text is U+002D HYPHEN-MINUS (-),
		// consume it. Otherwise, this is an invalid <urange>.
		if text[0] != '-' {
			return false
		}
		text = text[1:]

		// Consume as many hex digits as possible from text. If zero hex
		// digits were consumed, or more than 6 hex digits were consumed,
		// or there are any code points left in text, this is an invalid
		// <urange>.
		digits := hexDigitsPrefix(text)
		if digits == 0 || digits > maxURangeDigits || digits != len(text) {
			return false
		}
		end = parseHex(text)
	}

	// If end value is greater than the maximum allowed code point, or
	// start value is greater than end value, the <urange> is invalid.
	return end <= unicode.MaxRune && start <= end
}

// hexDigitsPrefix returns the number of hex digits at the start of s.
func hexDigitsPrefix(s string) int {
	n := 0
	for n < len(s) && tokenizer.IsHexDigit(int(s[n])) {
		n++
	}
	return n
}

// parseHex returns the value of at most six hex digits.
func parseHex(digits string) int64 {
	var value int64
	for i := 0; i < len(digits); i++ {
		digit := int64(digits[i] - '0')
		if c := digits[i] | 0x20; c >= 'a' { // a letter, lowercased
			digit = int64(c-'a') + 10
		}
		value = value<<4 | digit
	}
	return value
}
