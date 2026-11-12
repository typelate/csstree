// Ported from lib/tokenizer/char-code-definitions.js.

package tokenizer

import "strings"

// Char codes are ints. In Go they are bytes of a UTF-8 string (see CharCodeAt);
// upstream reads UTF-16 code units. Every byte >= 0x80 is part of a non-ASCII
// code point, so it classifies the same way as the code unit upstream sees.

// https://drafts.csswg.org/css-syntax-3/
// § 4.2. Definitions

// IsDigit reports whether code is a digit.
//
// digit
// A code point between U+0030 DIGIT ZERO (0) and U+0039 DIGIT NINE (9).
func IsDigit(code int) bool {
	return '0' <= code && code <= '9'
}

// IsHexDigit reports whether code is a hex digit.
//
// hex digit
// A digit, or a code point between U+0041 LATIN CAPITAL LETTER A (A) and U+0046 LATIN CAPITAL LETTER F (F),
// or a code point between U+0061 LATIN SMALL LETTER A (a) and U+0066 LATIN SMALL LETTER F (f).
func IsHexDigit(code int) bool {
	return IsDigit(code) || // 0 .. 9
		('A' <= code && code <= 'F') ||
		('a' <= code && code <= 'f')
}

// isUppercaseLetter reports whether code is an uppercase letter.
//
// uppercase letter
// A code point between U+0041 LATIN CAPITAL LETTER A (A) and U+005A LATIN CAPITAL LETTER Z (Z).
func isUppercaseLetter(code int) bool {
	return 'A' <= code && code <= 'Z'
}

// isLowercaseLetter reports whether code is a lowercase letter.
//
// lowercase letter
// A code point between U+0061 LATIN SMALL LETTER A (a) and U+007A LATIN SMALL LETTER Z (z).
func isLowercaseLetter(code int) bool {
	return 'a' <= code && code <= 'z'
}

// isLetter reports whether code is a letter.
//
// letter
// An uppercase letter or a lowercase letter.
func isLetter(code int) bool {
	return isUppercaseLetter(code) || isLowercaseLetter(code)
}

// isNonASCII reports whether code is a non-ASCII code point.
//
// non-ASCII code point
// A code point with a value equal to or greater than U+0080 <control>.
//
// 2024-09-02: The latest spec narrows the range for non-ASCII characters (see https://github.com/csstree/csstree/issues/188).
// However, all modern browsers support a wider range, and strictly following the latest spec could result
// in some CSS being parsed incorrectly, even though it works in the browser. Therefore, this function adheres
// to the previous, broader definition of non-ASCII characters.
func isNonASCII(code int) bool {
	return code >= 0x80
}

// isNameStart reports whether code is a name-start code point.
//
// name-start code point
// A letter, a non-ASCII code point, or U+005F LOW LINE (_).
func isNameStart(code int) bool {
	return isLetter(code) || isNonASCII(code) || code == '_'
}

// IsName reports whether code is a name code point.
//
// name code point
// A name-start code point, a digit, or U+002D HYPHEN-MINUS (-).
func IsName(code int) bool {
	return isNameStart(code) || IsDigit(code) || code == '-'
}

// isNonPrintable reports whether code is a non-printable code point.
//
// non-printable code point
// A code point between U+0000 NULL and U+0008 BACKSPACE, or U+000B LINE TABULATION,
// or a code point between U+000E SHIFT OUT and U+001F INFORMATION SEPARATOR ONE, or U+007F DELETE.
func isNonPrintable(code int) bool {
	return (0x00 <= code && code <= 0x08) ||
		code == '\v' ||
		(0x0E <= code && code <= 0x1F) ||
		code == 0x7F
}

// isNewline reports whether code is a newline.
//
// newline
// U+000A LINE FEED. Note that U+000D CARRIAGE RETURN and U+000C FORM FEED are not included in this definition,
// as they are converted to U+000A LINE FEED during preprocessing.
// The tokenizer does not preprocess its input, so CR and FF are newlines too.
func isNewline(code int) bool {
	return code == '\n' || code == '\r' || code == '\f'
}

// IsWhiteSpace reports whether code is whitespace.
//
// whitespace
// A newline, U+0009 CHARACTER TABULATION, or U+0020 SPACE.
func IsWhiteSpace(code int) bool {
	return isNewline(code) || code == ' ' || code == '\t'
}

// IsValidEscape reports whether two code points are a valid escape.
//
// § 4.3.8. Check if two code points are a valid escape
func IsValidEscape(first, second int) bool {
	// If the first code point is not U+005C REVERSE SOLIDUS (\), return false.
	if first != '\\' {
		return false
	}

	// Otherwise, if the second code point is a newline, return false.
	// (upstream also returns false for EOF; the spec does not, so a \ at
	// the end of the source is an escape of U+FFFD)
	if isNewline(second) {
		return false
	}

	// Otherwise, return true.
	return true
}

// IsIdentifierStart reports whether three code points would start an identifier.
//
// § 4.3.9. Check if three code points would start an identifier
func IsIdentifierStart(first, second, third int) bool {
	// Look at the first code point:

	// U+002D HYPHEN-MINUS
	if first == '-' {
		// If the second code point is a name-start code point or a U+002D HYPHEN-MINUS,
		// or the second and third code points are a valid escape, return true. Otherwise, return false.
		return isNameStart(second) ||
			second == '-' ||
			IsValidEscape(second, third)
	}

	// name-start code point
	if isNameStart(first) {
		// Return true.
		return true
	}

	// U+005C REVERSE SOLIDUS (\)
	if first == '\\' {
		// If the first and second code points are a valid escape, return true. Otherwise, return false.
		return IsValidEscape(first, second)
	}

	// anything else
	// Return false.
	return false
}

// isNumberStart reports whether three code points would start a number.
//
// § 4.3.10. Check if three code points would start a number
func isNumberStart(first, second, third int) bool {
	// Look at the first code point:

	// U+002B PLUS SIGN (+)
	// U+002D HYPHEN-MINUS (-)
	if first == '+' || first == '-' {
		// If the second code point is a digit, return true.
		// Otherwise, if the second code point is a U+002E FULL STOP (.)
		// and the third code point is a digit, return true.
		// Otherwise, return false.
		return IsDigit(second) || second == '.' && IsDigit(third)
	}

	// U+002E FULL STOP (.)
	if first == '.' {
		// If the second code point is a digit, return true. Otherwise, return false.
		return IsDigit(second)
	}

	// digit
	// Return true.
	// anything else
	// Return false.
	return IsDigit(first)
}

//
// Misc
//

// bomLen returns the length of the byte order mark
// (https://en.wikipedia.org/wiki/Byte_order_mark) at the start of source,
// or 0 when there is none. Upstream's isBOM(code) returns 1, the number of
// UTF-16 code units; U+FEFF is 3 bytes of UTF-8.
//
// Only U+FEFF is a BOM. A Go string is already decoded, and a UTF-16LE BOM
// decodes to U+FEFF too; upstream also skips U+FFFE, a noncharacter that CSS
// Syntax §3.2 treats as ordinary input.
func bomLen(source string) int {
	const bom = "\uFEFF"
	if strings.HasPrefix(source, bom) {
		return len(bom)
	}

	return 0
}

// Fast code category
// Only ASCII code points have a special meaning, so the table covers 0..127
// only; the category values are above that range.
const (
	eofCategory          = 0x80
	whiteSpaceCategory   = 0x82
	digitCategory        = 0x83
	nameStartCategory    = 0x84
	nonPrintableCategory = 0x85
)

// category is a read-only table built once at package initialization.
var category = func() (category [0x80]int) {
	for i := range category {
		switch {
		case IsWhiteSpace(i):
			category[i] = whiteSpaceCategory
		case IsDigit(i):
			category[i] = digitCategory
		case isNameStart(i):
			category[i] = nameStartCategory
		case isNonPrintable(i):
			category[i] = nonPrintableCategory
		case i != 0:
			category[i] = i
		default:
			category[i] = eofCategory
		}
	}
	return category
}()

// charCodeCategory returns one of the *Category constants for code, or code
// itself when it has no category.
func charCodeCategory(code int) int {
	switch {
	case code < 0:
		return code
	case code < len(category):
		return category[code]
	default:
		return nameStartCategory
	}
}
