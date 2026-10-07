// Ported from lib/tokenizer/index.js.

// Package tokenizer splits CSS text into tokens as CSS Syntax Module Level 3
// describes. It is the tokenizer of package csstree, ported from csstree's
// lib/tokenizer.
//
// [Tokenize] calls a function for each token with its [TokenType] and the
// byte offsets of its text. A [TokenStream] holds the tokens of a source
// and a cursor over them; the csstree parser embeds one. Its
// [TokenStream.SkipUntilBalanced] skips to the end of a block, asking a
// [StopFunc] at each token whether to stop there, as a [StopMode] says.
//
// Offsets count bytes of the UTF-8 source, not UTF-16 code units as in
// JavaScript. [OffsetToLocation] converts offsets to lines and columns:
// [OffsetToLocation.Position] returns a [Position] and
// [OffsetToLocation.Range] a [Location]. Columns count bytes too.
//
// The tokenizer does not copy or rewrite the source. It reads a NUL byte
// and invalid UTF-8 as U+FFFD, as CSS Syntax preprocessing does, and
// treats CR, FF and CRLF as newlines. The char code helpers, such as
// [CharCodeAt], [IsName] and [ConsumeEscaped], are the ones the parser and
// lexer of package csstree share.
package tokenizer

import "strings"

// Tokenize splits source into tokens and calls onToken for each one with its
// type and the byte offsets [start, end) of its text. A leading byte order
// mark is skipped. Tokens cover the rest of source without gaps.
//
// Tokenize reads source as CSS Syntax §3.3 preprocessing leaves it, without
// copying it: a NUL byte is U+FFFD REPLACEMENT CHARACTER, a name code point,
// and so is any invalid UTF-8 (every byte >= 0x80 is a name code point).
// Offsets still index the original bytes, so a token's text keeps the NUL or
// invalid bytes; DecodeEscaped and the csstree Decode functions turn them
// into U+FFFD. CR, FF and CRLF are newlines without being rewritten.
func Tokenize(source string, onToken func(tokenType TokenType, start, end int)) {
	// upstream: isBOM(getCharCode(0))
	s := scanner{source: source, offset: bomLen(source)}

	for start := s.offset; s.offset < len(source); start = s.offset {
		tokenType := s.consumeToken()

		// put token to stream
		onToken(tokenType, start, s.offset)
	}
}

// scanner is the state of a Tokenize call: the source and the offset of the
// next code point to consume. Each consume method consumes one token from
// offset on and returns its type.
type scanner struct {
	source string
	offset int
}

// charCode reads U+FFFD for a NUL byte (§ 3.3. Preprocessing the input
// stream) and 0 for EOF; see CharCodeAt.
func (s *scanner) charCode(offset int) int {
	return CharCodeAt(s.source, offset)
}

// delim consumes the current code point as a <delim-token>.
func (s *scanner) delim() TokenType {
	s.offset++
	return Delim
}

// https://drafts.csswg.org/css-syntax-3/#consume-token
// § 4.3.1. Consume a token
func (s *scanner) consumeToken() TokenType {
	code := s.charCode(s.offset)

	switch charCodeCategory(code) {
	// whitespace
	case whiteSpaceCategory:
		// Consume as much whitespace as possible. Return a <whitespace-token>.
		s.offset = findWhiteSpaceEnd(s.source, s.offset+1)
		return WhiteSpace

	// U+0022 QUOTATION MARK (")
	// U+0027 APOSTROPHE (')
	case '"', '\'':
		// Consume a string token and return it.
		return s.consumeStringToken()

	// U+0023 NUMBER SIGN (#)
	case '#':
		// If the next input code point is a name code point or the next two input code points are a valid escape, then:
		if IsName(s.charCode(s.offset+1)) || IsValidEscape(s.charCode(s.offset+1), s.charCode(s.offset+2)) {
			// Create a <hash-token>.
			// If the next 3 input code points would start an identifier, set the <hash-token>’s type flag to "id".
			// (the type flag is not stored)
			// Consume a name, and set the <hash-token>’s value to the returned string.
			s.offset = consumeName(s.source, s.offset+1)

			// Return the <hash-token>.
			return Hash
		}

		// Otherwise, return a <delim-token> with its value set to the current input code point.
		return s.delim()

	// U+0028 LEFT PARENTHESIS (()
	case '(':
		// Return a <(-token>.
		s.offset++
		return LeftParenthesis

	// U+0029 RIGHT PARENTHESIS ())
	case ')':
		// Return a <)-token>.
		s.offset++
		return RightParenthesis

	// U+002B PLUS SIGN (+)
	// U+002E FULL STOP (.)
	case '+', '.':
		// If the input stream starts with a number, ...
		if isNumberStart(code, s.charCode(s.offset+1), s.charCode(s.offset+2)) {
			// ... reconsume the current input code point, consume a numeric token, and return it.
			return s.consumeNumericToken()
		}

		// Otherwise, return a <delim-token> with its value set to the current input code point.
		return s.delim()

	// U+002C COMMA (,)
	case ',':
		// Return a <comma-token>.
		s.offset++
		return Comma

	// U+002D HYPHEN-MINUS (-)
	case '-':
		// If the input stream starts with a number, reconsume the current input code point, consume a numeric token, and return it.
		if isNumberStart(code, s.charCode(s.offset+1), s.charCode(s.offset+2)) {
			return s.consumeNumericToken()
		}

		// Otherwise, if the next 2 input code points are U+002D HYPHEN-MINUS U+003E GREATER-THAN SIGN (->), consume them and return a <CDC-token>.
		if s.charCode(s.offset+1) == '-' && s.charCode(s.offset+2) == '>' {
			s.offset += 3
			return CDC
		}

		// Otherwise, if the input stream starts with an identifier, ...
		if IsIdentifierStart(code, s.charCode(s.offset+1), s.charCode(s.offset+2)) {
			// ... reconsume the current input code point, consume an ident-like token, and return it.
			return s.consumeIdentLikeToken()
		}

		// Otherwise, return a <delim-token> with its value set to the current input code point.
		return s.delim()

	// U+002F SOLIDUS (/)
	case '/':
		// If the next two input code point are U+002F SOLIDUS (/) followed by a U+002A ASTERISK (*),
		if s.charCode(s.offset+1) == '*' {
			// ... consume them and all following code points up to and including the first U+002A ASTERISK (*)
			// followed by a U+002F SOLIDUS (/), or up to an EOF code point.
			if i := strings.Index(s.source[s.offset+2:], "*/"); i == -1 {
				s.offset = len(s.source)
			} else {
				s.offset += 2 + i + 2
			}
			return Comment
		}

		return s.delim()

	// U+003A COLON (:)
	case ':':
		// Return a <colon-token>.
		s.offset++
		return Colon

	// U+003B SEMICOLON (;)
	case ';':
		// Return a <semicolon-token>.
		s.offset++
		return Semicolon

	// U+003C LESS-THAN SIGN (<)
	case '<':
		// If the next 3 input code points are U+0021 EXCLAMATION MARK U+002D HYPHEN-MINUS U+002D HYPHEN-MINUS (!--), ...
		if s.charCode(s.offset+1) == '!' &&
			s.charCode(s.offset+2) == '-' &&
			s.charCode(s.offset+3) == '-' {
			// ... consume them and return a <CDO-token>.
			s.offset += 4
			return CDO
		}

		// Otherwise, return a <delim-token> with its value set to the current input code point.
		return s.delim()

	// U+0040 COMMERCIAL AT (@)
	case '@':
		// If the next 3 input code points would start an identifier, ...
		if IsIdentifierStart(s.charCode(s.offset+1), s.charCode(s.offset+2), s.charCode(s.offset+3)) {
			// ... consume a name, create an <at-keyword-token> with its value set to the returned value, and return it.
			s.offset = consumeName(s.source, s.offset+1)
			return AtKeyword
		}

		// Otherwise, return a <delim-token> with its value set to the current input code point.
		return s.delim()

	// U+005B LEFT SQUARE BRACKET ([)
	case '[':
		// Return a <[-token>.
		s.offset++
		return LeftSquareBracket

	// U+005C REVERSE SOLIDUS (\)
	case '\\':
		// If the input stream starts with a valid escape, ...
		if IsValidEscape(code, s.charCode(s.offset+1)) {
			// ... reconsume the current input code point, consume an ident-like token, and return it.
			return s.consumeIdentLikeToken()
		}

		// Otherwise, this is a parse error. Return a <delim-token> with its value set to the current input code point.
		return s.delim()

	// U+005D RIGHT SQUARE BRACKET (])
	case ']':
		// Return a <]-token>.
		s.offset++
		return RightSquareBracket

	// U+007B LEFT CURLY BRACKET ({)
	case '{':
		// Return a <{-token>.
		s.offset++
		return LeftCurlyBracket

	// U+007D RIGHT CURLY BRACKET (})
	case '}':
		// Return a <}-token>.
		s.offset++
		return RightCurlyBracket

	// digit
	case digitCategory:
		// Reconsume the current input code point, consume a numeric token, and return it.
		return s.consumeNumericToken()

	// name-start code point
	case nameStartCategory:
		// Reconsume the current input code point, consume an ident-like token, and return it.
		return s.consumeIdentLikeToken()

	// EOF: Tokenize stops before it, so there is no <EOF-token>.

	// anything else
	default:
		// Return a <delim-token> with its value set to the current input code point.
		return s.delim()
	}
}

// § 4.3.3. Consume a numeric token
func (s *scanner) consumeNumericToken() TokenType {
	// Consume a number and let number be the result.
	s.offset = ConsumeNumber(s.source, s.offset)

	// If the next 3 input code points would start an identifier, then:
	if IsIdentifierStart(s.charCode(s.offset), s.charCode(s.offset+1), s.charCode(s.offset+2)) {
		// Create a <dimension-token> with the same value and type flag as number, and a unit set initially to the empty string.
		// Consume a name. Set the <dimension-token>’s unit to the returned value.
		// Return the <dimension-token>.
		s.offset = consumeName(s.source, s.offset)
		return Dimension
	}

	// Otherwise, if the next input code point is U+0025 PERCENTAGE SIGN (%), consume it.
	if s.charCode(s.offset) == '%' {
		// Create a <percentage-token> with the same value as number, and return it.
		s.offset++
		return Percentage
	}

	// Otherwise, create a <number-token> with the same value and type flag as number, and return it.
	return Number
}

// § 4.3.4. Consume an ident-like token
func (s *scanner) consumeIdentLikeToken() TokenType {
	nameStartOffset := s.offset

	// Consume a name, and let string be the result.
	s.offset = consumeName(s.source, s.offset)

	// If string’s value is an ASCII case-insensitive match for "url",
	// and the next input code point is U+0028 LEFT PARENTHESIS ((), consume it.
	if CmpStr(s.source, nameStartOffset, s.offset, "url") && s.charCode(s.offset) == '(' {
		// While the next two input code points are whitespace, consume the next input code point.
		s.offset = findWhiteSpaceEnd(s.source, s.offset+1)

		// If the next one or two input code points are U+0022 QUOTATION MARK ("), U+0027 APOSTROPHE ('),
		// or whitespace followed by U+0022 QUOTATION MARK (") or U+0027 APOSTROPHE ('),
		// then create a <function-token> with its value set to string and return it.
		if code := s.charCode(s.offset); code == '"' || code == '\'' {
			s.offset = nameStartOffset + len("url(")
			return Function
		}

		// Otherwise, consume a url token, and return it.
		return s.consumeURLToken()
	}

	// Otherwise, if the next input code point is U+0028 LEFT PARENTHESIS ((), consume it.
	// Create a <function-token> with its value set to string and return it.
	if s.charCode(s.offset) == '(' {
		s.offset++
		return Function
	}

	// Otherwise, create an <ident-token> with its value set to string and return it.
	return Ident
}

// § 4.3.5. Consume a string token
//
// The ending code point is the current input code point (a " or a '),
// which this consumes first.
func (s *scanner) consumeStringToken() TokenType {
	endingCodePoint := s.charCode(s.offset)
	s.offset++

	// Initially create a <string-token> with its value set to the empty string.
	// Repeatedly consume the next input code point from the stream:
	for ; s.offset < len(s.source); s.offset++ {
		code := s.charCode(s.offset)

		switch charCodeCategory(code) {
		// ending code point
		case endingCodePoint:
			// Return the <string-token>.
			s.offset++
			return String

		// EOF: the loop ends; this is a parse error. Return the <string-token>.

		// newline
		case whiteSpaceCategory:
			if isNewline(code) {
				// This is a parse error. Reconsume the current input code point,
				// create a <bad-string-token>, and return it.
				// (upstream consumes the newline into the bad-string; the
				// port leaves it for the next token, as the spec says)
				return BadString
			}

		// U+005C REVERSE SOLIDUS (\)
		case '\\':
			// If the next input code point is EOF, do nothing.
			if s.offset == len(s.source)-1 {
				break
			}

			nextCode := s.charCode(s.offset + 1)

			// Otherwise, if the next input code point is a newline, consume it.
			if isNewline(nextCode) {
				s.offset += newlineLength(s.source, s.offset+1)
			} else if IsValidEscape(code, nextCode) {
				// Otherwise, (the stream starts with a valid escape) consume
				// an escaped code point and append the returned code point to
				// the <string-token>’s value.
				s.offset = ConsumeEscaped(s.source, s.offset) - 1
			}

			// anything else
			// Append the current input code point to the <string-token>’s value.
		}
	}

	return String
}

// § 4.3.6. Consume a url token
// Note: This algorithm assumes that the initial "url(" has already been consumed.
// This algorithm also assumes that it’s being called to consume an "unquoted" value, like url(foo).
// A quoted value, like url("foo"), is parsed as a <function-token>. Consume an ident-like token
// automatically handles this distinction; this algorithm shouldn’t be called directly otherwise.
func (s *scanner) consumeURLToken() TokenType {
	// Initially create a <url-token> with its value set to the empty string.

	// Consume as much whitespace as possible.
	s.offset = findWhiteSpaceEnd(s.source, s.offset)

	// Repeatedly consume the next input code point from the stream:
	for ; s.offset < len(s.source); s.offset++ {
		code := s.charCode(s.offset)

		switch charCodeCategory(code) {
		// U+0029 RIGHT PARENTHESIS ())
		case ')':
			// Return the <url-token>.
			s.offset++
			return URL

		// EOF: the loop ends; this is a parse error. Return the <url-token>.

		// whitespace
		case whiteSpaceCategory:
			// Consume as much whitespace as possible.
			s.offset = findWhiteSpaceEnd(s.source, s.offset)

			// If the next input code point is U+0029 RIGHT PARENTHESIS ()) or EOF,
			// consume it and return the <url-token>
			// (if EOF was encountered, this is a parse error);
			if s.offset >= len(s.source) {
				return URL
			}
			if s.charCode(s.offset) == ')' {
				s.offset++
				return URL
			}

			// otherwise, consume the remnants of a bad url, create a <bad-url-token>,
			// and return it.
			s.offset = consumeBadURLRemnants(s.source, s.offset)
			return BadURL

		// U+0022 QUOTATION MARK (")
		// U+0027 APOSTROPHE (')
		// U+0028 LEFT PARENTHESIS (()
		// non-printable code point
		case '"', '\'', '(', nonPrintableCategory:
			// This is a parse error. Consume the remnants of a bad url,
			// create a <bad-url-token>, and return it.
			s.offset = consumeBadURLRemnants(s.source, s.offset)
			return BadURL

		// U+005C REVERSE SOLIDUS (\)
		case '\\':
			// If the stream starts with a valid escape, consume an escaped code point and
			// append the returned code point to the <url-token>’s value.
			if IsValidEscape(code, s.charCode(s.offset+1)) {
				s.offset = ConsumeEscaped(s.source, s.offset) - 1
				break
			}

			// Otherwise, this is a parse error. Consume the remnants of a bad url,
			// create a <bad-url-token>, and return it.
			s.offset = consumeBadURLRemnants(s.source, s.offset)
			return BadURL

			// anything else
			// Append the current input code point to the <url-token>’s value.
		}
	}

	return URL
}
