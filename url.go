// Ported from lib/utils/url.js.

package csstree

import (
	"strconv"
	"strings"

	"github.com/typelate/csstree/tokenizer"
)

const (
	space            = 0x0020 // U+0020 SPACE
	leftParenthesis  = 0x0028 // U+0028 LEFT PARENTHESIS (()
	rightParenthesis = 0x0029 // U+0029 RIGHT PARENTHESIS ())
)

// DecodeURL returns the URL of a CSS url token such as `url(foo)`, with
// whitespace trimmed and escapes resolved (upstream url.decode). NUL and
// invalid UTF-8, escaped or not, decode to U+FFFD as CSS Syntax §3.3
// preprocessing replaces them.
func DecodeURL(str string) string {
	length := len(str)
	start := 4 // length of "url("
	end := length - 1
	if tokenizer.CharCodeAt(str, length-1) == rightParenthesis {
		end = length - 2
	}

	for start < end && tokenizer.IsWhiteSpace(tokenizer.CharCodeAt(str, start)) {
		start++
	}

	for start < end && tokenizer.IsWhiteSpace(tokenizer.CharCodeAt(str, end)) {
		end--
	}

	decoded, trailingBackslash := decodeEscapes(str, start, end)
	if trailingBackslash && end != length-1 {
		// upstream: include the last right parenthesis as escaped
		return str[end+1:]
	}

	return decoded
}

// EncodeURL serializes str as a CSS url token `url(...)` (upstream
// url.encode).
func EncodeURL(str string) string {
	var encoded strings.Builder
	wsBeforeHexIsNeeded := false

	for i := 0; i < len(str); i++ {
		code := int(str[i])

		// If the character is NULL (U+0000), then the REPLACEMENT CHARACTER (U+FFFD).
		if code == 0x0000 {
			encoded.WriteString("\uFFFD")
			continue
		}

		// If the character is in the range [\1-\1f] (U+0001 to U+001F) or is U+007F,
		// the character escaped as code point.
		// Note: Do not compare with 0x0001 since 0x0000 is precessed before
		if code <= 0x001f || code == 0x007F {
			encoded.WriteString(`\` + strconv.FormatInt(int64(code), 16))
			wsBeforeHexIsNeeded = true
			continue
		}

		if code == space ||
			code == reverseSolidus ||
			code == quotationMark ||
			code == apostrophe ||
			code == leftParenthesis ||
			code == rightParenthesis {
			encoded.WriteString(`\` + str[i:i+1])
			wsBeforeHexIsNeeded = false
		} else {
			if wsBeforeHexIsNeeded && tokenizer.IsHexDigit(code) {
				encoded.WriteString(" ")
			}

			encoded.WriteByte(str[i])
			wsBeforeHexIsNeeded = false
		}
	}

	return "url(" + encoded.String() + ")"
}
