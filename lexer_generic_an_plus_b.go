// Ported from lib/lexer/generic-an-plus-b.js.

package csstree

import (
	"github.com/typelate/csstree/definitionsyntax"
	"github.com/typelate/csstree/tokenizer"
)

// isDelimToken reports whether token is the <delim-token> delim.
func isDelimToken(token *matchToken, delim rune) bool {
	return token != nil && token.typ == tokenizer.Delim && token.value == string(delim)
}

// skipSC returns the index of the first token from offset that is not
// whitespace or a comment, len(tokens) when there is none.
func skipSC(tokens []matchToken, offset int) int {
	for offset < len(tokens) && (tokens[offset].typ == tokenizer.WhiteSpace || tokens[offset].typ == tokenizer.Comment) {
		offset++
	}

	return offset
}

// isSign reports whether c is + or -.
func isSign(c int) bool {
	return c == '+' || c == '-'
}

// isSignlessInteger reports whether s is decimal digits only; "" is.
func isSignlessInteger(s string) bool {
	for i := 0; i < len(s); i++ {
		if !tokenizer.IsDigit(int(s[i])) {
			return false
		}
	}

	return true
}

// isSignedInteger reports whether s is decimal digits with an optional
// sign.
func isSignedInteger(s string) bool {
	if s != "" && isSign(int(s[0])) {
		s = s[1:]
	}

	return isSignlessInteger(s)
}

// signlessIntegerAfter returns how many tokens An+B takes when it ends with
// a <signless-integer> after the first offset tokens, whitespace and
// comments: up to and including the integer, 0 when there is none.
func signlessIntegerAfter(tokens []matchToken, offset int) int {
	offset = skipSC(tokens, offset)
	if offset == len(tokens) || !isSignlessInteger(tokens[offset].value) {
		// Integer is expected
		return 0
	}

	return offset + 1
}

// withB returns how many tokens An+B takes when its An takes the first
// offset tokens, followed by an optional B after whitespace and comments:
//
//	... <signed-integer>
//	... ['+' | '-'] <signless-integer>
//
// It returns 0 when B is not well formed.
func withB(tokens []matchToken, offset int) int {
	next := skipSC(tokens, offset)
	if next == len(tokens) {
		return offset
	}

	token := &tokens[next]

	// ... <signed-integer>
	if token.typ == tokenizer.Number {
		if !isSign(tokenizer.CharCodeAt(token.value, 0)) {
			// Number sign is expected
			return 0
		}
		if !isSignedInteger(token.value[1:]) {
			// Integer is expected
			return 0
		}
		return next + 1
	}

	// ... ['+' | '-'] <signless-integer>
	if !isDelimToken(token, '+') && !isDelimToken(token, '-') {
		return offset
	}

	next = skipSC(tokens, next+1)
	if next == len(tokens) || tokens[next].typ != tokenizer.Number || !isSignlessInteger(tokens[next].value) {
		return 0
	}

	return next + 1
}

// anPlusB is the generic type <an-plus-b>, the An+B microsyntax
// https://www.w3.org/TR/css-syntax-3/#anb.
func anPlusB(tokens []matchToken, _ *definitionsyntax.Range) int {
	if len(tokens) == 0 {
		return 0
	}

	token := &tokens[0]
	switch {
	// <integer>
	case token.typ == tokenizer.Number:
		if !isSignedInteger(token.value) {
			return 0
		}
		return 1 // b

	case token.typ == tokenizer.Ident && tokenizer.CharCodeAt(token.value, 0) == '-':
		return dashNIdent(tokens)

	case token.typ == tokenizer.Ident:
		return nIdent(tokens, 0)

	// just ignore a plus
	case isDelimToken(token, '+') && isIdentToken(tokenAt(tokens, 1)):
		return nIdent(tokens, 1)

	case token.typ == tokenizer.Dimension:
		return nDimension(tokens)
	}

	return 0
}

// dashNIdent matches An+B starting with an ident starting with -:
//
//	-n
//	-n <signed-integer>
//	-n ['+' | '-'] <signless-integer>
//	-n- <signless-integer>
//	<dashndashdigit-ident>
func dashNIdent(tokens []matchToken) int {
	value := tokens[0].value

	// expect 1st char is N
	if !tokenizer.CmpChar(value, 1, 'n') {
		return 0
	}

	// -n
	// -n <signed-integer>
	// -n ['+' | '-'] <signless-integer>
	if len(value) == 2 {
		return withB(tokens, 1)
	}

	if tokenizer.CharCodeAt(value, 2) != '-' {
		return 0
	}

	// -n- <signless-integer>
	if len(value) == 3 {
		return signlessIntegerAfter(tokens, 1)
	}

	// <dashndashdigit-ident>
	if !isSignlessInteger(value[3:]) {
		return 0
	}

	return 1
}

// nIdent matches An+B whose n ident is tokens[offset], after an optional
// '+':
//
//	'+'? n
//	'+'? n <signed-integer>
//	'+'? n ['+' | '-'] <signless-integer>
//	'+'? n- <signless-integer>
//	'+'? <ndashdigit-ident>
func nIdent(tokens []matchToken, offset int) int {
	value := tokens[offset].value

	if !tokenizer.CmpChar(value, 0, 'n') {
		return 0
	}

	// '+'? n
	// '+'? n <signed-integer>
	// '+'? n ['+' | '-'] <signless-integer>
	if len(value) == 1 {
		return withB(tokens, offset+1)
	}

	if tokenizer.CharCodeAt(value, 1) != '-' {
		return 0
	}

	// '+'? n- <signless-integer>
	if len(value) == 2 {
		return signlessIntegerAfter(tokens, offset+1)
	}

	// '+'? <ndashdigit-ident>
	if !isSignlessInteger(value[2:]) {
		return 0
	}

	return offset + 1
}

// nDimension matches An+B starting with a dimension:
//
//	<ndashdigit-dimension>
//	<ndash-dimension> <signless-integer>
//	<n-dimension>
//	<n-dimension> <signed-integer>
//	<n-dimension> ['+' | '-'] <signless-integer>
func nDimension(tokens []matchToken) int {
	value := tokens[0].value

	digitsStart := 0
	if isSign(tokenizer.CharCodeAt(value, 0)) {
		digitsStart = 1
	}
	i := digitsStart
	for i < len(value) && tokenizer.IsDigit(int(value[i])) {
		i++
	}

	if i == digitsStart {
		// Integer is expected
		return 0
	}

	if !tokenizer.CmpChar(value, i, 'n') {
		return 0
	}

	// <n-dimension>
	// <n-dimension> <signed-integer>
	// <n-dimension> ['+' | '-'] <signless-integer>
	if i+1 == len(value) {
		return withB(tokens, 1)
	}

	if tokenizer.CharCodeAt(value, i+1) != '-' {
		return 0
	}

	// <ndash-dimension> <signless-integer>
	if i+2 == len(value) {
		return signlessIntegerAfter(tokens, 1)
	}

	// <ndashdigit-dimension>
	if !isSignlessInteger(value[i+2:]) {
		return 0
	}

	return 1
}

// isIdentToken is upstream's getNextToken(offset + 1).type === Ident, which
// throws a TypeError when there is no next token; here a missing token is
// not an ident.
func isIdentToken(token *matchToken) bool {
	return token != nil && token.typ == tokenizer.Ident
}
