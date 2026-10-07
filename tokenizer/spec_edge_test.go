package tokenizer

// Edge cases from CSS Syntax Module Level 3 (§3.3 preprocessing, §4.2
// definitions, §4.3 tokenization). Every expected value is what upstream
// csstree 3.2.1 produces for the same input; where upstream departs from the
// spec the case name says so, and the port keeps upstream's behaviour unless
// spec_fixes_test.go covers it.

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func tokenizeChunks(source string) []tokenChunk {
	var chunks []tokenChunk
	Tokenize(source, func(tokenType TokenType, start, end int) {
		chunks = append(chunks, tokenChunk{Type: tokenType.String(), Chunk: source[start:end]})
	})
	return chunks
}

func TestTokenizeSpecEdges(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
		want   []tokenChunk
	}{
		{"§4.3.5 escaped CRLF continues the string", "\"a\\\r\nb\"", []tokenChunk{{"string-token", "\"a\\\r\nb\""}}},
		{"§4.3.5 escaped LF continues the string", "'a\\\nb'", []tokenChunk{{"string-token", "'a\\\nb'"}}},
		{"§4.3.5 an escape is consumed whole", "\"\\61\" a", []tokenChunk{{"string-token", "\"\\61\""}, {"whitespace-token", " "}, {"ident-token", "a"}}},
		{"§4.3.5 an escaped quote does not end the string", "'\\'' a", []tokenChunk{{"string-token", "'\\''"}, {"whitespace-token", " "}, {"ident-token", "a"}}},
		{"§4.3.5 backslash at EOF", "\"\\", []tokenChunk{{"string-token", "\"\\"}}},
		{"§4.3.5 EOF ends a string", "'abc", []tokenChunk{{"string-token", "'abc"}}},
		{"§4.3.6 whitespace then EOF in a url", "url(a ", []tokenChunk{{"url-token", "url(a "}}},
		{"§4.3.6 whitespace then ) in a url", "url( a\n\t) b", []tokenChunk{{"url-token", "url( a\n\t)"}, {"whitespace-token", " "}, {"ident-token", "b"}}},
		{"§4.3.6 whitespace inside a url makes it bad", "url(a b)c", []tokenChunk{{"bad-url-token", "url(a b)"}, {"ident-token", "c"}}},
		{"§4.3.6 a quote inside a url makes it bad", "url(a\"b)c", []tokenChunk{{"bad-url-token", "url(a\"b)"}, {"ident-token", "c"}}},
		{"§4.3.6 an apostrophe inside a url makes it bad", "url(a'b)c", []tokenChunk{{"bad-url-token", "url(a'b)"}, {"ident-token", "c"}}},
		{"§4.3.6 ( inside a url makes it bad", "url(a(b)c", []tokenChunk{{"bad-url-token", "url(a(b)"}, {"ident-token", "c"}}},
		{"§4.3.6 a non-printable code point makes a url bad", "url(a\u000bb)c", []tokenChunk{{"bad-url-token", "url(a\u000bb)"}, {"ident-token", "c"}}},
		{"§4.3.6 DEL makes a url bad", "url(a\u007fb)c", []tokenChunk{{"bad-url-token", "url(a\u007fb)"}, {"ident-token", "c"}}},
		{"§4.3.6 an escaped ) stays in a url", "url(a\\)b)c", []tokenChunk{{"url-token", "url(a\\)b)"}, {"ident-token", "c"}}},
		{"§4.3.6 an escaped newline makes a url bad", "url(a\\\nb)c", []tokenChunk{{"bad-url-token", "url(a\\\nb)"}, {"ident-token", "c"}}},
		{"§4.3.6 backslash at EOF makes a url bad", "url(a\\", []tokenChunk{{"bad-url-token", "url(a\\"}}},
		{"§4.3.6 url( followed by whitespace and a quote is a function", "url(  \"a\")", []tokenChunk{{"function-token", "url("}, {"whitespace-token", "  "}, {"string-token", "\"a\""}, {")-token", ")"}}},
		{"§4.3.6 url( is ASCII case-insensitive", "URL(a)uRl(b)", []tokenChunk{{"url-token", "URL(a)"}, {"url-token", "uRl(b)"}}},
		{"§4.3.6 an empty url", "url()", []tokenChunk{{"url-token", "url()"}}},
		{"§4.3.14 bad url remnants stop at )", "url(a\"b)c)d", []tokenChunk{{"bad-url-token", "url(a\"b)"}, {"ident-token", "c"}, {")-token", ")"}, {"ident-token", "d"}}},
		{"§4.3.14 an escape after a newline in bad url remnants", "url(a\"\n\\)b)c", []tokenChunk{{"bad-url-token", "url(a\"\n\\)b)"}, {"ident-token", "c"}}},
		{"§4.3.14 an escaped ) in bad url remnants", "url(a\"\\)b)c", []tokenChunk{{"bad-url-token", "url(a\"\\)b)"}, {"ident-token", "c"}}},
		{"§4.3.12 digits", "123 4.56 .7 +.8 -.9", []tokenChunk{{"number-token", "123"}, {"whitespace-token", " "}, {"number-token", "4.56"}, {"whitespace-token", " "}, {"number-token", ".7"}, {"whitespace-token", " "}, {"number-token", "+.8"}, {"whitespace-token", " "}, {"number-token", "-.9"}}},
		{"§4.3.12 a sign or dot alone is a delim", "+.a +a -. .", []tokenChunk{{"delim-token", "+"}, {"delim-token", "."}, {"ident-token", "a"}, {"whitespace-token", " "}, {"delim-token", "+"}, {"ident-token", "a"}, {"whitespace-token", " "}, {"delim-token", "-"}, {"delim-token", "."}, {"whitespace-token", " "}, {"delim-token", "."}}},
		{"§4.3.12 exponents", "1e5 1E5 1e-5 1e+5 1.5e3", []tokenChunk{{"number-token", "1e5"}, {"whitespace-token", " "}, {"number-token", "1E5"}, {"whitespace-token", " "}, {"number-token", "1e-5"}, {"whitespace-token", " "}, {"number-token", "1e+5"}, {"whitespace-token", " "}, {"number-token", "1.5e3"}}},
		{"§4.3.12 an e that is not an exponent starts a unit", "1ex 1em 1e 1e- 1e+x 1a2", []tokenChunk{{"dimension-token", "1ex"}, {"whitespace-token", " "}, {"dimension-token", "1em"}, {"whitespace-token", " "}, {"dimension-token", "1e"}, {"whitespace-token", " "}, {"dimension-token", "1e-"}, {"whitespace-token", " "}, {"dimension-token", "1e"}, {"delim-token", "+"}, {"ident-token", "x"}, {"whitespace-token", " "}, {"dimension-token", "1a2"}}},
		{"§4.3.12 percentage", "1% -1.5%", []tokenChunk{{"percentage-token", "1%"}, {"whitespace-token", " "}, {"percentage-token", "-1.5%"}}},
		{"§4.3.12 a dot without digits ends the number", "1. 1.e1", []tokenChunk{{"number-token", "1"}, {"delim-token", "."}, {"whitespace-token", " "}, {"number-token", "1"}, {"delim-token", "."}, {"ident-token", "e1"}}},
		{"§4.3.3 dimension units with escapes and dashes", "1\\31 1-a 1--a 1-\\31 1é", []tokenChunk{{"dimension-token", "1\\31 1-a"}, {"whitespace-token", " "}, {"dimension-token", "1--a"}, {"whitespace-token", " "}, {"dimension-token", "1-\\31 1é"}}},
		{"§4.3.3 a lone dash after a number is a delim", "1-", []tokenChunk{{"number-token", "1"}, {"delim-token", "-"}}},
		{"§4.3.2 comments", "/* c */a/**/b/*/c", []tokenChunk{{"comment-token", "/* c */"}, {"ident-token", "a"}, {"comment-token", "/**/"}, {"ident-token", "b"}, {"comment-token", "/*/c"}}},
		{"§4.3.2 an unterminated comment runs to EOF", "a/*b", []tokenChunk{{"ident-token", "a"}, {"comment-token", "/*b"}}},
		{"§4.3.1 a slash not followed by * is a delim", "/x", []tokenChunk{{"delim-token", "/"}, {"ident-token", "x"}}},
		{"§4.3.1 CDO", "<!--a<!-a<!a<a", []tokenChunk{{"CDO-token", "<!--"}, {"ident-token", "a"}, {"delim-token", "<"}, {"delim-token", "!"}, {"ident-token", "-a"}, {"delim-token", "<"}, {"delim-token", "!"}, {"ident-token", "a"}, {"delim-token", "<"}, {"ident-token", "a"}}},
		{"§4.3.1 < is CDO only before !--", "<a--<!a-<-!-", []tokenChunk{{"delim-token", "<"}, {"ident-token", "a--"}, {"delim-token", "<"}, {"delim-token", "!"}, {"ident-token", "a-"}, {"delim-token", "<"}, {"delim-token", "-"}, {"delim-token", "!"}, {"delim-token", "-"}}},
		{"§4.3.1 CDC", "-->a--->", []tokenChunk{{"CDC-token", "-->"}, {"ident-token", "a---"}, {"delim-token", ">"}}},
		{"§4.3.1 hyphen", "-a --a -- - -1 -\\31 -é", []tokenChunk{{"ident-token", "-a"}, {"whitespace-token", " "}, {"ident-token", "--a"}, {"whitespace-token", " "}, {"ident-token", "--"}, {"whitespace-token", " "}, {"delim-token", "-"}, {"whitespace-token", " "}, {"number-token", "-1"}, {"whitespace-token", " "}, {"ident-token", "-\\31 -é"}}},
		{"§4.3.1 hyphen followed by an escaped newline", "-\\\n", []tokenChunk{{"delim-token", "-"}, {"delim-token", "\\"}, {"whitespace-token", "\n"}}},
		{"§4.3.1 hash", "#a #1 #- #\\31 #\\\n #é #", []tokenChunk{{"hash-token", "#a"}, {"whitespace-token", " "}, {"hash-token", "#1"}, {"whitespace-token", " "}, {"hash-token", "#-"}, {"whitespace-token", " "}, {"hash-token", "#\\31 "}, {"delim-token", "#"}, {"delim-token", "\\"}, {"whitespace-token", "\n "}, {"hash-token", "#é"}, {"whitespace-token", " "}, {"delim-token", "#"}}},
		{"§4.3.1 at-keyword", "@a @- @-- @-a @\\31 @1 @é @", []tokenChunk{{"at-keyword-token", "@a"}, {"whitespace-token", " "}, {"delim-token", "@"}, {"delim-token", "-"}, {"whitespace-token", " "}, {"at-keyword-token", "@--"}, {"whitespace-token", " "}, {"at-keyword-token", "@-a"}, {"whitespace-token", " "}, {"at-keyword-token", "@\\31 "}, {"delim-token", "@"}, {"number-token", "1"}, {"whitespace-token", " "}, {"at-keyword-token", "@é"}, {"whitespace-token", " "}, {"delim-token", "@"}}},
		{"§4.3.1 backslash followed by a newline is a delim", "\\\nb", []tokenChunk{{"delim-token", "\\"}, {"whitespace-token", "\n"}, {"ident-token", "b"}}},
		{"§4.3.1 an escape in an ident", "a\\62 c\\\"d", []tokenChunk{{"ident-token", "a\\62 c\\\"d"}}},
		{"§4.3.1 an ident ends at a backslash before EOF", "a\\", []tokenChunk{{"ident-token", "a"}, {"delim-token", "\\"}}},
		{"§4.3.7 at most six hex digits in an escape", "\\aaaaaaa", []tokenChunk{{"ident-token", "\\aaaaaaa"}}},
		{"§4.3.7 one whitespace after a hex escape", "\\61  \\61\r\nb", []tokenChunk{{"ident-token", "\\61 "}, {"whitespace-token", " "}, {"ident-token", "\\61\r\nb"}}},
		{"§4.3.1 function tokens", "a( é( \\61(", []tokenChunk{{"function-token", "a("}, {"whitespace-token", " "}, {"function-token", "é("}, {"whitespace-token", " "}, {"function-token", "\\61("}}},
		{"§4.3.1 blocks", "{[()]}", []tokenChunk{{"{-token", "{"}, {"[-token", "["}, {"(-token", "("}, {")-token", ")"}, {"]-token", "]"}, {"}-token", "}"}}},
		{"§4.3.1 punctuation", "a:b;c,d", []tokenChunk{{"ident-token", "a"}, {"colon-token", ":"}, {"ident-token", "b"}, {"semicolon-token", ";"}, {"ident-token", "c"}, {"comma-token", ","}, {"ident-token", "d"}}},
		{"§3.3 a leading BOM is skipped", "\ufeffa", []tokenChunk{{"ident-token", "a"}}},
		{"§3.3 only a leading BOM is skipped", "a\ufeffb", []tokenChunk{{"ident-token", "a\ufeffb"}}},
		{"non-ASCII code points are name code points", "é😀 \u0080\u00a0\u2028", []tokenChunk{{"ident-token", "é😀"}, {"whitespace-token", " "}, {"ident-token", "\u0080\u00a0\u2028"}}},
		{"DEL and C0 controls are delims", "\u007f\u0001", []tokenChunk{{"delim-token", "\u007f"}, {"delim-token", "\u0001"}}},
		// Go only: upstream sees U+FFFD for invalid UTF-8, a name code point
		// like every byte >= 0x80.
		{"invalid UTF-8 bytes are name code points", "\xff\xfe(", []tokenChunk{{"function-token", "\xff\xfe("}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenizeChunks(tt.source); !slices.Equal(got, tt.want) {
				t.Errorf("Tokenize(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

// Tokens nested 100000 deep balance without recursion.
func TestTokenStreamDeepNesting(t *testing.T) {
	const depth = 100000
	source := strings.Repeat("(", depth) + strings.Repeat(")", depth)
	ts := NewTokenStream(source)

	for _, tt := range []struct{ index, want int }{
		{0, 2*depth - 1},
		{depth - 1, depth},
		{depth, depth - 1},
		{2*depth - 1, 0},
	} {
		t.Run(fmt.Sprint(tt.index), func(t *testing.T) {
			if got := ts.GetBlockTokenPairIndex(tt.index); got != tt.want {
				t.Errorf("GetBlockTokenPairIndex(%d) = %d, want %d", tt.index, got, tt.want)
			}
		})
	}
}

func TestCharCodeDefinitions(t *testing.T) {
	for _, tt := range []struct {
		code                                                        int
		digit, hex, upper, lower, letter, nonASCII, nameStart, name bool
		nonPrintable, newline, whiteSpace                           bool
		category                                                    int
	}{
		{0x00, false, false, false, false, false, false, false, false, true, false, false, 0x85},
		{0x08, false, false, false, false, false, false, false, false, true, false, false, 0x85},
		{0x09, false, false, false, false, false, false, false, false, false, false, true, 0x82},
		{0x0a, false, false, false, false, false, false, false, false, false, true, true, 0x82},
		{0x0b, false, false, false, false, false, false, false, false, true, false, false, 0x85},
		{0x0c, false, false, false, false, false, false, false, false, false, true, true, 0x82},
		{0x0d, false, false, false, false, false, false, false, false, false, true, true, 0x82},
		{0x0e, false, false, false, false, false, false, false, false, true, false, false, 0x85},
		{0x1f, false, false, false, false, false, false, false, false, true, false, false, 0x85},
		{0x20, false, false, false, false, false, false, false, false, false, false, true, 0x82},
		{0x2d, false, false, false, false, false, false, false, true, false, false, false, 0x2d},
		{0x2f, false, false, false, false, false, false, false, false, false, false, false, 0x2f},
		{0x30, true, true, false, false, false, false, false, true, false, false, false, 0x83},
		{0x39, true, true, false, false, false, false, false, true, false, false, false, 0x83},
		{0x3a, false, false, false, false, false, false, false, false, false, false, false, 0x3a},
		{0x40, false, false, false, false, false, false, false, false, false, false, false, 0x40},
		{0x41, false, true, true, false, true, false, true, true, false, false, false, 0x84},
		{0x46, false, true, true, false, true, false, true, true, false, false, false, 0x84},
		{0x47, false, false, true, false, true, false, true, true, false, false, false, 0x84},
		{0x5a, false, false, true, false, true, false, true, true, false, false, false, 0x84},
		{0x5b, false, false, false, false, false, false, false, false, false, false, false, 0x5b},
		{0x5c, false, false, false, false, false, false, false, false, false, false, false, 0x5c},
		{0x5f, false, false, false, false, false, false, true, true, false, false, false, 0x84},
		{0x60, false, false, false, false, false, false, false, false, false, false, false, 0x60},
		{0x61, false, true, false, true, true, false, true, true, false, false, false, 0x84},
		{0x66, false, true, false, true, true, false, true, true, false, false, false, 0x84},
		{0x67, false, false, false, true, true, false, true, true, false, false, false, 0x84},
		{0x7a, false, false, false, true, true, false, true, true, false, false, false, 0x84},
		{0x7b, false, false, false, false, false, false, false, false, false, false, false, 0x7b},
		{0x7e, false, false, false, false, false, false, false, false, false, false, false, 0x7e},
		{0x7f, false, false, false, false, false, false, false, false, true, false, false, 0x85},
		{0x80, false, false, false, false, false, true, true, true, false, false, false, 0x84},
		{0xff, false, false, false, false, false, true, true, true, false, false, false, 0x84},
	} {
		t.Run(fmt.Sprintf("%#x", tt.code), func(t *testing.T) {
			for _, check := range []struct {
				name string
				fn   func(int) bool
				want bool
			}{
				{"IsDigit", IsDigit, tt.digit},
				{"IsHexDigit", IsHexDigit, tt.hex},
				{"isUppercaseLetter", isUppercaseLetter, tt.upper},
				{"isLowercaseLetter", isLowercaseLetter, tt.lower},
				{"isLetter", isLetter, tt.letter},
				{"isNonASCII", isNonASCII, tt.nonASCII},
				{"isNameStart", isNameStart, tt.nameStart},
				{"IsName", IsName, tt.name},
				{"isNonPrintable", isNonPrintable, tt.nonPrintable},
				{"isNewline", isNewline, tt.newline},
				{"IsWhiteSpace", IsWhiteSpace, tt.whiteSpace},
			} {
				if got := check.fn(tt.code); got != check.want {
					t.Errorf("%s(%#x) = %t, want %t", check.name, tt.code, got, check.want)
				}
			}
			if got := charCodeCategory(tt.code); got != tt.category {
				t.Errorf("charCodeCategory(%#x) = %#x, want %#x", tt.code, got, tt.category)
			}
		})
	}
}

// codesAt returns the char codes at 0, 1 and 2, with 0 past the end as the
// tokenizer's getCharCode does.
func codesAt(s string) (int, int, int) {
	return CharCodeAt(s, 0), CharCodeAt(s, 1), CharCodeAt(s, 2)
}

func TestIsNumberStart(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   bool
	}{
		{"+1", true},
		{"+.1", true},
		{"+.a", false},
		{"+a", false},
		{"-1", true},
		{"-.1", true},
		{"-.", false},
		{".1", true},
		{".a", false},
		{"1", true},
		{"a", false},
		{"+", false},
		{"", false},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := isNumberStart(codesAt(tt.source)); got != tt.want {
				t.Errorf("isNumberStart(%q) = %t, want %t", tt.source, got, tt.want)
			}
		})
	}
}

func TestIsIdentifierStart(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   bool
	}{
		{"-a", true},
		{"--", true},
		{"-\\a", true},
		{"-\\\n", false},
		{"-1", false},
		{"-", false},
		{"a", true},
		{"_", true},
		{"é", true},
		{"\\a", true},
		{"\\\n", false},
		{"\\", false},
		{"1", false},
		{"", false},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := IsIdentifierStart(codesAt(tt.source)); got != tt.want {
				t.Errorf("IsIdentifierStart(%q) = %t, want %t", tt.source, got, tt.want)
			}
		})
	}
}

func TestBOMLen(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   int
	}{
		{"\ufeff", 3}, // upstream 1: one UTF-16 code unit; three bytes of UTF-8
		{"\ufeffa", 3},
		{"a\ufeff", 0},
		{"\ufffe", 0}, // upstream 1: it takes U+FFFE for a little-endian BOM
		{"\ufffd", 0},
		{"\xef\xbb", 0},
		{"a", 0},
		{"", 0},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if got := bomLen(tt.source); got != tt.want {
				t.Errorf("bomLen(%q) = %d, want %d", tt.source, got, tt.want)
			}
		})
	}
}

func TestNewlineLength(t *testing.T) {
	for _, tt := range []struct {
		source       string
		offset, want int
	}{
		{"\r\n", 0, 2}, {"\n\n", 0, 1}, {"\r\r", 0, 1}, {"\r", 0, 1}, {"\n", 0, 1}, {"a\r\n", 1, 2}, {"\f\n", 0, 1},
	} {
		t.Run(fmt.Sprintf("%q@%d", tt.source, tt.offset), func(t *testing.T) {
			if got := newlineLength(tt.source, tt.offset); got != tt.want {
				t.Errorf("newlineLength(%q, %d) = %d, want %d", tt.source, tt.offset, got, tt.want)
			}
		})
	}
}

func TestCmpChar(t *testing.T) {
	for _, tt := range []struct {
		source            string
		offset, reference int
		want              bool
	}{
		{"e", 0, 'e', true},
		{"E", 0, 'e', true},
		{"x", 0, 'e', false},
		{"@", 0, '`', false},
		{"[", 0, '{', false},
		{"A", 0, 'a', true},
		{"Z", 0, 'z', true},
		{"", 0, 'e', false},
	} {
		t.Run(fmt.Sprintf("%q@%d", tt.source, tt.offset), func(t *testing.T) {
			if got := CmpChar(tt.source, tt.offset, tt.reference); got != tt.want {
				t.Errorf("CmpChar(%q, %d, %q) = %t, want %t", tt.source, tt.offset, rune(tt.reference), got, tt.want)
			}
		})
	}
}

func TestCmpStr(t *testing.T) {
	for _, tt := range []struct {
		source, reference string
		start, end        int
		want              bool
	}{
		{"url", "url", 0, 3, true},
		{"URL", "url", 0, 3, true},
		{"xurl", "url", 1, 4, true},
		{"url", "url", -1, 2, false},
		{"ur", "url", 0, 3, false},
		{"url", "url", 0, 2, false},
		{"urm", "url", 0, 3, false},
		{"@[`", "`{@", 0, 3, false},
		{"", "", 0, 0, true},
	} {
		t.Run(fmt.Sprintf("%q[%d:%d]", tt.source, tt.start, tt.end), func(t *testing.T) {
			if got := CmpStr(tt.source, tt.start, tt.end, tt.reference); got != tt.want {
				t.Errorf("CmpStr(%q, %d, %d, %q) = %t, want %t", tt.source, tt.start, tt.end, tt.reference, got, tt.want)
			}
		})
	}
}

func TestFindWhiteSpace(t *testing.T) {
	for _, tt := range []struct {
		source       string
		offset, want int
	}{
		{" a", 0, 0}, {"  a", 1, 0}, {"a  ", 2, 1}, {"a", 0, 1}, {"", 0, 1}, {"\n\t\r\f a", 4, 0}, {"a b", 1, 1},
	} {
		t.Run(fmt.Sprintf("start %q@%d", tt.source, tt.offset), func(t *testing.T) {
			if got := FindWhiteSpaceStart(tt.source, tt.offset); got != tt.want {
				t.Errorf("FindWhiteSpaceStart(%q, %d) = %d, want %d", tt.source, tt.offset, got, tt.want)
			}
		})
	}

	for _, tt := range []struct {
		source                    string
		offset, wantWS, wantDigit int
	}{
		{"  a", 0, 2, 0}, {"a", 0, 0, 0}, {"  ", 0, 2, 0}, {"123a", 0, 0, 3}, {"123", 1, 1, 3}, {"", 0, 0, 0},
	} {
		t.Run(fmt.Sprintf("end %q@%d", tt.source, tt.offset), func(t *testing.T) {
			if got := findWhiteSpaceEnd(tt.source, tt.offset); got != tt.wantWS {
				t.Errorf("findWhiteSpaceEnd(%q, %d) = %d, want %d", tt.source, tt.offset, got, tt.wantWS)
			}
			if got := findDecimalNumberEnd(tt.source, tt.offset); got != tt.wantDigit {
				t.Errorf("findDecimalNumberEnd(%q, %d) = %d, want %d", tt.source, tt.offset, got, tt.wantDigit)
			}
		})
	}
}

func TestConsumeHelpers(t *testing.T) {
	for _, tt := range []struct {
		name   string
		fn     func(string, int) int
		source string
		want   int
	}{
		{"ConsumeEscaped", ConsumeEscaped, "\\a", 2},
		{"ConsumeEscaped", ConsumeEscaped, "\\aaaaaaa", 7},
		{"ConsumeEscaped", ConsumeEscaped, "\\aaaaaa", 7},
		{"ConsumeEscaped", ConsumeEscaped, "\\abcdef0", 7},
		{"ConsumeEscaped", ConsumeEscaped, "\\x", 2},
		{"ConsumeEscaped", ConsumeEscaped, "\\a b", 3},
		{"ConsumeEscaped", ConsumeEscaped, "\\a\r\nb", 4},
		{"ConsumeEscaped", ConsumeEscaped, "\\a\nb", 3},
		{"ConsumeEscaped", ConsumeEscaped, "\\aa", 3},
		{"ConsumeEscaped", ConsumeEscaped, "\\g", 2},
		{"consumeName", consumeName, "abc d", 3},
		{"consumeName", consumeName, "a\\62 c", 6},
		{"consumeName", consumeName, "a\\", 1},
		{"consumeName", consumeName, "a\\\nb", 1},
		{"consumeName", consumeName, "-_1é(", 5}, // upstream 4: é is two bytes
		{"consumeName", consumeName, "a\\)b", 4},
		{"ConsumeNumber", ConsumeNumber, "1", 1},
		{"ConsumeNumber", ConsumeNumber, "123", 3},
		{"ConsumeNumber", ConsumeNumber, "+1", 2},
		{"ConsumeNumber", ConsumeNumber, "-1.5", 4},
		{"ConsumeNumber", ConsumeNumber, ".5", 2},
		{"ConsumeNumber", ConsumeNumber, "1.", 1},
		{"ConsumeNumber", ConsumeNumber, "1.5.5", 3},
		{"ConsumeNumber", ConsumeNumber, "1e5", 3},
		{"ConsumeNumber", ConsumeNumber, "1E5", 3},
		{"ConsumeNumber", ConsumeNumber, "1e+5", 4},
		{"ConsumeNumber", ConsumeNumber, "1e-5", 4},
		{"ConsumeNumber", ConsumeNumber, "1e", 1},
		{"ConsumeNumber", ConsumeNumber, "1e+", 1},
		{"ConsumeNumber", ConsumeNumber, "1ex", 1},
		{"ConsumeNumber", ConsumeNumber, "1a2", 1},
		{"ConsumeNumber", ConsumeNumber, "1.5e-3x", 6},
		{"ConsumeNumber", ConsumeNumber, "+", 1},
		{"ConsumeNumber", ConsumeNumber, "a", 0},
		{"consumeBadURLRemnants", consumeBadURLRemnants, "a)b", 2},
		{"consumeBadURLRemnants", consumeBadURLRemnants, "ab", 2},
		{"consumeBadURLRemnants", consumeBadURLRemnants, ")", 1},
		{"consumeBadURLRemnants", consumeBadURLRemnants, "\\)a)b", 4},
		{"consumeBadURLRemnants", consumeBadURLRemnants, "\\", 1},
		{"consumeBadURLRemnants", consumeBadURLRemnants, "a\\", 2},
		{"consumeBadURLRemnants", consumeBadURLRemnants, "\\aa)b)", 4}, // upstream 6: it skips the ) after an escape
		{"consumeBadURLRemnants", consumeBadURLRemnants, "\\a)b)", 3},  // upstream 5
		{"consumeBadURLRemnants", consumeBadURLRemnants, "\n\\)b)c", 5},
		{"consumeBadURLRemnants", consumeBadURLRemnants, "", 0},
	} {
		t.Run(fmt.Sprintf("%s(%q)", tt.name, tt.source), func(t *testing.T) {
			if got := tt.fn(tt.source, 0); got != tt.want {
				t.Errorf("%s(%q, 0) = %d, want %d", tt.name, tt.source, got, tt.want)
			}
		})
	}
}

func TestDecodeEscapedSpec(t *testing.T) {
	// § 4.3.7: zero, a surrogate or a code point past U+10FFFF is U+FFFD.
	for _, tt := range []struct{ escaped, want string }{
		{"x", "x"},
		{"(", "("},
		{"é", "é"},
		{"41", "A"},
		{"4A", "J"},
		{"4a", "J"},
		{"a", "\n"},
		{"A", "\n"},
		{"F", "\u000f"},
		{"f", "\u000f"},
		{"0", "\ufffd"},
		{"000000", "\ufffd"},
		{"d7ff", "\ud7ff"},
		{"D800", "\ufffd"},
		{"dfff", "\ufffd"},
		{"DFFF", "\ufffd"},
		{"e000", "\ue000"},
		{"fffd", "\ufffd"},
		{"10ffff", "\U0010ffff"},
		{"10FFFF", "\U0010ffff"},
		{"110000", "\ufffd"},
		{"ffffff", "\ufffd"},
		{"1f600", "😀"},
		{"41 ", "A"},
		{"41\n", "A"},
	} {
		t.Run(tt.escaped, func(t *testing.T) {
			if got := DecodeEscaped(tt.escaped); got != tt.want {
				t.Errorf("DecodeEscaped(%q) = %q, want %q", tt.escaped, got, tt.want)
			}
		})
	}
}

// tokenStreamSource has every kind of block, a comment, delims and a
// function; the tables below are upstream's TokenStream results for it.
const tokenStreamSource = "a ( [b /*c*/] .d|e) {x: f(1)} . "

func TestTokenStreamLookups(t *testing.T) {
	for _, tt := range []struct {
		index             int
		lookupType        [4]TokenType // offsets -1, 0, 1, 2
		lookupTypeNonSC   [3]TokenType // 0, 1, 2
		lookupOffset      [4]int       // -1, 0, 1, 2
		lookupOffsetNonSC [3]int       // 0, 1, 2
		lookupValueB      [2]bool      // 0, 1
		isBalanceEdge     [4]bool      // index-1, index, index+1, index+5
		isDelimDot        [3]bool      // 0, 1, 2
		substrToCursor    string
	}{
		{index: 0, lookupType: [4]TokenType{EOF, Ident, WhiteSpace, LeftParenthesis}, lookupTypeNonSC: [3]TokenType{Ident, LeftParenthesis, LeftSquareBracket}, lookupOffset: [4]int{0, 0, 1, 2}, lookupOffsetNonSC: [3]int{0, 2, 4}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, false}, isDelimDot: [3]bool{false, false, false}, substrToCursor: ""},
		{index: 1, lookupType: [4]TokenType{Ident, WhiteSpace, LeftParenthesis, WhiteSpace}, lookupTypeNonSC: [3]TokenType{LeftParenthesis, LeftSquareBracket, Ident}, lookupOffset: [4]int{0, 1, 2, 3}, lookupOffsetNonSC: [3]int{1, 3, 4}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, false}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a"},
		{index: 2, lookupType: [4]TokenType{WhiteSpace, LeftParenthesis, WhiteSpace, LeftSquareBracket}, lookupTypeNonSC: [3]TokenType{LeftParenthesis, LeftSquareBracket, Ident}, lookupOffset: [4]int{1, 2, 3, 4}, lookupOffsetNonSC: [3]int{0, 2, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, false}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a "},
		{index: 3, lookupType: [4]TokenType{LeftParenthesis, WhiteSpace, LeftSquareBracket, Ident}, lookupTypeNonSC: [3]TokenType{LeftSquareBracket, Ident, RightSquareBracket}, lookupOffset: [4]int{2, 3, 4, 5}, lookupOffsetNonSC: [3]int{1, 2, 5}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, false}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ("},
		{index: 4, lookupType: [4]TokenType{WhiteSpace, LeftSquareBracket, Ident, WhiteSpace}, lookupTypeNonSC: [3]TokenType{LeftSquareBracket, Ident, RightSquareBracket}, lookupOffset: [4]int{3, 4, 5, 6}, lookupOffsetNonSC: [3]int{0, 1, 4}, lookupValueB: [2]bool{false, true}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( "},
		{index: 5, lookupType: [4]TokenType{LeftSquareBracket, Ident, WhiteSpace, Comment}, lookupTypeNonSC: [3]TokenType{Ident, RightSquareBracket, Delim}, lookupOffset: [4]int{4, 5, 6, 7}, lookupOffsetNonSC: [3]int{0, 3, 5}, lookupValueB: [2]bool{true, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( ["},
		{index: 6, lookupType: [4]TokenType{Ident, WhiteSpace, Comment, RightSquareBracket}, lookupTypeNonSC: [3]TokenType{RightSquareBracket, Delim, Ident}, lookupOffset: [4]int{5, 6, 7, 12}, lookupOffsetNonSC: [3]int{2, 4, 5}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b"},
		{index: 7, lookupType: [4]TokenType{WhiteSpace, Comment, RightSquareBracket, WhiteSpace}, lookupTypeNonSC: [3]TokenType{RightSquareBracket, Delim, Ident}, lookupOffset: [4]int{6, 7, 12, 13}, lookupOffsetNonSC: [3]int{1, 3, 4}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b "},
		{index: 8, lookupType: [4]TokenType{Comment, RightSquareBracket, WhiteSpace, Delim}, lookupTypeNonSC: [3]TokenType{RightSquareBracket, Delim, Ident}, lookupOffset: [4]int{7, 12, 13, 14}, lookupOffsetNonSC: [3]int{0, 2, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{true, true, true, true}, isDelimDot: [3]bool{false, false, true}, substrToCursor: "a ( [b /*c*/"},
		{index: 9, lookupType: [4]TokenType{RightSquareBracket, WhiteSpace, Delim, Ident}, lookupTypeNonSC: [3]TokenType{Delim, Ident, Delim}, lookupOffset: [4]int{12, 13, 14, 15}, lookupOffsetNonSC: [3]int{1, 2, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, false}, isDelimDot: [3]bool{false, true, false}, substrToCursor: "a ( [b /*c*/]"},
		{index: 10, lookupType: [4]TokenType{WhiteSpace, Delim, Ident, Delim}, lookupTypeNonSC: [3]TokenType{Delim, Ident, Delim}, lookupOffset: [4]int{13, 14, 15, 16}, lookupOffsetNonSC: [3]int{0, 1, 2}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{true, false, false}, substrToCursor: "a ( [b /*c*/] "},
		{index: 11, lookupType: [4]TokenType{Delim, Ident, Delim, Ident}, lookupTypeNonSC: [3]TokenType{Ident, Delim, Ident}, lookupOffset: [4]int{14, 15, 16, 17}, lookupOffsetNonSC: [3]int{0, 1, 2}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] ."},
		{index: 12, lookupType: [4]TokenType{Ident, Delim, Ident, RightParenthesis}, lookupTypeNonSC: [3]TokenType{Delim, Ident, RightParenthesis}, lookupOffset: [4]int{15, 16, 17, 18}, lookupOffsetNonSC: [3]int{0, 1, 2}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d"},
		{index: 13, lookupType: [4]TokenType{Delim, Ident, RightParenthesis, WhiteSpace}, lookupTypeNonSC: [3]TokenType{Ident, RightParenthesis, LeftCurlyBracket}, lookupOffset: [4]int{16, 17, 18, 19}, lookupOffsetNonSC: [3]int{0, 1, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|"},
		{index: 14, lookupType: [4]TokenType{Ident, RightParenthesis, WhiteSpace, LeftCurlyBracket}, lookupTypeNonSC: [3]TokenType{RightParenthesis, LeftCurlyBracket, Ident}, lookupOffset: [4]int{17, 18, 19, 20}, lookupOffsetNonSC: [3]int{0, 2, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{true, true, true, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e"},
		{index: 15, lookupType: [4]TokenType{RightParenthesis, WhiteSpace, LeftCurlyBracket, Ident}, lookupTypeNonSC: [3]TokenType{LeftCurlyBracket, Ident, Colon}, lookupOffset: [4]int{18, 19, 20, 21}, lookupOffsetNonSC: [3]int{1, 2, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, false}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e)"},
		{index: 16, lookupType: [4]TokenType{WhiteSpace, LeftCurlyBracket, Ident, Colon}, lookupTypeNonSC: [3]TokenType{LeftCurlyBracket, Ident, Colon}, lookupOffset: [4]int{19, 20, 21, 22}, lookupOffsetNonSC: [3]int{0, 1, 2}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, false}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) "},
		{index: 17, lookupType: [4]TokenType{LeftCurlyBracket, Ident, Colon, WhiteSpace}, lookupTypeNonSC: [3]TokenType{Ident, Colon, Function}, lookupOffset: [4]int{20, 21, 22, 23}, lookupOffsetNonSC: [3]int{0, 1, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, false}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) {"},
		{index: 18, lookupType: [4]TokenType{Ident, Colon, WhiteSpace, Function}, lookupTypeNonSC: [3]TokenType{Colon, Function, Number}, lookupOffset: [4]int{21, 22, 23, 24}, lookupOffsetNonSC: [3]int{0, 2, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, false}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) {x"},
		{index: 19, lookupType: [4]TokenType{Colon, WhiteSpace, Function, Number}, lookupTypeNonSC: [3]TokenType{Function, Number, RightParenthesis}, lookupOffset: [4]int{22, 23, 24, 26}, lookupOffsetNonSC: [3]int{1, 2, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) {x:"},
		{index: 20, lookupType: [4]TokenType{WhiteSpace, Function, Number, RightParenthesis}, lookupTypeNonSC: [3]TokenType{Function, Number, RightParenthesis}, lookupOffset: [4]int{23, 24, 26, 27}, lookupOffsetNonSC: [3]int{0, 1, 2}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) {x: "},
		{index: 21, lookupType: [4]TokenType{Function, Number, RightParenthesis, RightCurlyBracket}, lookupTypeNonSC: [3]TokenType{Number, RightParenthesis, RightCurlyBracket}, lookupOffset: [4]int{24, 26, 27, 28}, lookupOffsetNonSC: [3]int{0, 1, 2}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) {x: f("},
		{index: 22, lookupType: [4]TokenType{Number, RightParenthesis, RightCurlyBracket, WhiteSpace}, lookupTypeNonSC: [3]TokenType{RightParenthesis, RightCurlyBracket, Delim}, lookupOffset: [4]int{26, 27, 28, 29}, lookupOffsetNonSC: [3]int{0, 1, 3}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{true, true, true, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) {x: f(1"},
		{index: 23, lookupType: [4]TokenType{RightParenthesis, RightCurlyBracket, WhiteSpace, Delim}, lookupTypeNonSC: [3]TokenType{RightCurlyBracket, Delim, EOF}, lookupOffset: [4]int{27, 28, 29, 30}, lookupOffsetNonSC: [3]int{0, 2, 0}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{true, true, true, true}, isDelimDot: [3]bool{false, false, true}, substrToCursor: "a ( [b /*c*/] .d|e) {x: f(1)"},
		{index: 24, lookupType: [4]TokenType{RightCurlyBracket, WhiteSpace, Delim, WhiteSpace}, lookupTypeNonSC: [3]TokenType{Delim, EOF, EOF}, lookupOffset: [4]int{28, 29, 30, 31}, lookupOffsetNonSC: [3]int{1, 0, 0}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, true, false}, substrToCursor: "a ( [b /*c*/] .d|e) {x: f(1)}"},
		{index: 25, lookupType: [4]TokenType{WhiteSpace, Delim, WhiteSpace, EOF}, lookupTypeNonSC: [3]TokenType{Delim, EOF, EOF}, lookupOffset: [4]int{29, 30, 31, 32}, lookupOffsetNonSC: [3]int{0, 0, 0}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{true, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) {x: f(1)} "},
		{index: 26, lookupType: [4]TokenType{Delim, WhiteSpace, EOF, EOF}, lookupTypeNonSC: [3]TokenType{EOF, EOF, EOF}, lookupOffset: [4]int{30, 31, 32, 32}, lookupOffsetNonSC: [3]int{0, 0, 0}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, false, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) {x: f(1)} ."},
		{index: 27, lookupType: [4]TokenType{WhiteSpace, EOF, EOF, EOF}, lookupTypeNonSC: [3]TokenType{EOF, EOF, EOF}, lookupOffset: [4]int{31, 32, 32, 32}, lookupOffsetNonSC: [3]int{0, 0, 0}, lookupValueB: [2]bool{false, false}, isBalanceEdge: [4]bool{false, false, true, true}, isDelimDot: [3]bool{false, false, false}, substrToCursor: "a ( [b /*c*/] .d|e) {x: f(1)} . "},
	} {
		t.Run(fmt.Sprint(tt.index), func(t *testing.T) {
			ts := NewTokenStream(tokenStreamSource)
			ts.Skip(tt.index)
			if ts.TokenIndex != tt.index {
				t.Fatalf("after Skip(%d) TokenIndex = %d", tt.index, ts.TokenIndex)
			}
			for i, offset := range []int{-1, 0, 1, 2} {
				if got := ts.LookupType(offset); got != tt.lookupType[i] {
					t.Errorf("at %d LookupType(%d) = %v, want %v", tt.index, offset, got, tt.lookupType[i])
				}
				if got := ts.LookupOffset(offset); got != tt.lookupOffset[i] {
					t.Errorf("at %d LookupOffset(%d) = %d, want %d", tt.index, offset, got, tt.lookupOffset[i])
				}
			}
			for idx := range 3 {
				if got := ts.LookupTypeNonSC(idx); got != tt.lookupTypeNonSC[idx] {
					t.Errorf("at %d LookupTypeNonSC(%d) = %v, want %v", tt.index, idx, got, tt.lookupTypeNonSC[idx])
				}
				if got := ts.LookupOffsetNonSC(idx); got != tt.lookupOffsetNonSC[idx] {
					t.Errorf("at %d LookupOffsetNonSC(%d) = %d, want %d", tt.index, idx, got, tt.lookupOffsetNonSC[idx])
				}
				if got := ts.IsDelim('.', idx); got != tt.isDelimDot[idx] {
					t.Errorf("at %d IsDelim('.', %d) = %t, want %t", tt.index, idx, got, tt.isDelimDot[idx])
				}
			}
			for offset := range 2 {
				if got := ts.LookupValue(offset, "b"); got != tt.lookupValueB[offset] {
					t.Errorf("at %d LookupValue(%d, %q) = %t, want %t", tt.index, offset, "b", got, tt.lookupValueB[offset])
				}
			}
			for i, tokenIndex := range []int{tt.index - 1, tt.index, tt.index + 1, tt.index + 5} {
				if got := ts.IsBalanceEdge(tokenIndex); got != tt.isBalanceEdge[i] {
					t.Errorf("at %d IsBalanceEdge(%d) = %t, want %t", tt.index, tokenIndex, got, tt.isBalanceEdge[i])
				}
			}
			if got := ts.SubstrToCursor(0); got != tt.substrToCursor {
				t.Errorf("at %d SubstrToCursor(0) = %q, want %q", tt.index, got, tt.substrToCursor)
			}
		})
	}
}

func TestTokenStreamAccessors(t *testing.T) {
	ts := NewTokenStream(tokenStreamSource)
	for _, tt := range []struct {
		index, start, end int
		tokenType         TokenType
		pair              int
	}{
		{-1, 0, 1, Ident, -1},
		{0, 0, 1, Ident, -1},
		{1, 1, 2, WhiteSpace, -1},
		{2, 2, 3, LeftParenthesis, 14},
		{3, 3, 4, WhiteSpace, -1},
		{4, 4, 5, LeftSquareBracket, 8},
		{5, 5, 6, Ident, -1},
		{6, 6, 7, WhiteSpace, -1},
		{7, 7, 12, Comment, -1},
		{8, 12, 13, RightSquareBracket, 4},
		{9, 13, 14, WhiteSpace, -1},
		{10, 14, 15, Delim, -1},
		{11, 15, 16, Ident, -1},
		{12, 16, 17, Delim, -1},
		{13, 17, 18, Ident, -1},
		{14, 18, 19, RightParenthesis, 2},
		{15, 19, 20, WhiteSpace, -1},
		{16, 20, 21, LeftCurlyBracket, 23},
		{17, 21, 22, Ident, -1},
		{18, 22, 23, Colon, -1},
		{19, 23, 24, WhiteSpace, -1},
		{20, 24, 26, Function, 22},
		{21, 26, 27, Number, -1},
		{22, 27, 28, RightParenthesis, 20},
		{23, 28, 29, RightCurlyBracket, 16},
		{24, 29, 30, WhiteSpace, -1},
		{25, 30, 31, Delim, -1},
		{26, 31, 32, WhiteSpace, -1},
		{27, 32, 32, EOF, -1},
		{28, 32, 32, EOF, -1},
	} {
		t.Run(fmt.Sprint(tt.index), func(t *testing.T) {
			if got := ts.GetTokenStart(tt.index); got != tt.start {
				t.Errorf("GetTokenStart(%d) = %d, want %d", tt.index, got, tt.start)
			}
			if got := ts.GetTokenEnd(tt.index); got != tt.end {
				t.Errorf("GetTokenEnd(%d) = %d, want %d", tt.index, got, tt.end)
			}
			if got := ts.GetTokenType(tt.index); got != tt.tokenType {
				t.Errorf("GetTokenType(%d) = %v, want %v", tt.index, got, tt.tokenType)
			}
			if got := ts.GetBlockTokenPairIndex(tt.index); got != tt.pair {
				t.Errorf("GetBlockTokenPairIndex(%d) = %d, want %d", tt.index, got, tt.pair)
			}
			wantOpener := tt.tokenType == Function || tt.tokenType == LeftParenthesis || tt.tokenType == LeftSquareBracket || tt.tokenType == LeftCurlyBracket
			if got := tt.tokenType.IsBlockOpener(); got != wantOpener {
				t.Errorf("%v.IsBlockOpener() = %t, want %t", tt.tokenType, got, wantOpener)
			}
			wantCloser := tt.tokenType == RightParenthesis || tt.tokenType == RightSquareBracket || tt.tokenType == RightCurlyBracket
			if got := tt.tokenType.IsBlockCloser(); got != wantCloser {
				t.Errorf("%v.IsBlockCloser() = %t, want %t", tt.tokenType, got, wantCloser)
			}
		})
	}
}

// Next and Skip report the same start and end for every token.
func TestTokenStreamNextAndSkip(t *testing.T) {
	ts := NewTokenStream(tokenStreamSource)
	for index := 0; !ts.EOF; index++ {
		skipped := NewTokenStream(tokenStreamSource)
		skipped.Skip(index)
		if ts.TokenStart != skipped.TokenStart || ts.TokenEnd != skipped.TokenEnd || ts.TokenType != skipped.TokenType {
			t.Errorf("Next to %d = %v [%d, %d), Skip(%d) = %v [%d, %d)", index, ts.TokenType, ts.TokenStart, ts.TokenEnd, index, skipped.TokenType, skipped.TokenStart, skipped.TokenEnd)
		}
		if want := ts.GetTokenEnd(index + 1); ts.TokenEnd > want || ts.TokenEnd < ts.TokenStart {
			t.Errorf("Next to %d end %d, next token ends at %d", index, ts.TokenEnd, want)
		}
		ts.Next()
	}
	if ts.TokenStart != len(tokenStreamSource) || ts.TokenEnd != len(tokenStreamSource) {
		t.Errorf("at EOF [%d, %d), want [%d, %d)", ts.TokenStart, ts.TokenEnd, len(tokenStreamSource), len(tokenStreamSource))
	}
}

// After Reset the cursor is before the first token; upstream answers
// lookups there (reading the missing cell -1 as type 0) instead of failing.
func TestTokenStreamLookupsAfterReset(t *testing.T) {
	ts := NewTokenStream(tokenStreamSource)
	ts.Reset()

	if got := ts.LookupTypeNonSC(0); got != EOF {
		t.Errorf("LookupTypeNonSC(0) = %v, want %v", got, EOF)
	}
	if got := ts.LookupTypeNonSC(1); got != Ident {
		t.Errorf("LookupTypeNonSC(1) = %v, want %v", got, Ident)
	}
	if got := ts.LookupOffsetNonSC(0); got != 0 {
		t.Errorf("LookupOffsetNonSC(0) = %d, want 0", got)
	}
	if got := ts.LookupOffsetNonSC(1); got != 1 {
		t.Errorf("LookupOffsetNonSC(1) = %d, want 1", got)
	}
	if got := ts.LookupType(0); got != EOF {
		t.Errorf("LookupType(0) = %v, want %v", got, EOF)
	}
	if got := ts.LookupType(1); got != Ident {
		t.Errorf("LookupType(1) = %v, want %v", got, Ident)
	}
	if got := ts.LookupOffset(1); got != 0 {
		t.Errorf("LookupOffset(1) = %d, want 0", got)
	}
	if got := ts.IsBalanceEdge(0); got {
		t.Errorf("IsBalanceEdge(0) = %t, want false", got)
	}
	if got := ts.GetTokenStart(-1); got != 0 {
		t.Errorf("GetTokenStart(-1) = %d, want 0", got)
	}
	if got := ts.GetBlockTokenPairIndex(-1); got != -1 {
		t.Errorf("GetBlockTokenPairIndex(-1) = %d, want -1", got)
	}
}

// A negative index reads token 0's type; upstream then reads the missing
// balance cell -1 and finds no pair.
func TestTokenStreamPairOfNegativeIndex(t *testing.T) {
	for _, source := range []string{"(a)", ")a("} {
		ts := NewTokenStream(source)
		ts.Next()
		if got := ts.GetBlockTokenPairIndex(-1); got != -1 {
			t.Errorf("NewTokenStream(%q).GetBlockTokenPairIndex(-1) = %d, want -1", source, got)
		}
	}
}

// SubstrToCursor swaps its ends as String.prototype.substring does.
func TestSubstrToCursorAfterCursor(t *testing.T) {
	ts := NewTokenStream(tokenStreamSource)
	ts.Skip(2)
	if got, want := ts.SubstrToCursor(5), "( ["; got != want {
		t.Errorf("SubstrToCursor(5) at offset %d = %q, want %q", ts.TokenStart, got, want)
	}
	if got, want := ts.SubstrToCursor(100), tokenStreamSource[2:]; got != want {
		t.Errorf("SubstrToCursor(100) = %q, want %q", got, want)
	}
}

func TestTokenStreamUnbalanced(t *testing.T) {
	for _, tt := range []struct {
		source  string
		balance []int
		pairs   []int
	}{
		{"(([)]", []int{5, 5, 4, 4, 2}, []int{-1, -1, 4, -1, 2, -1}},
		{")]}{[(", []int{6, 6, 6, 6, 6, 6}, []int{-1, -1, -1, -1, -1, -1, -1}},
	} {
		t.Run(tt.source, func(t *testing.T) {
			ts := NewTokenStream(tt.source)
			var balance []int
			for _, token := range ts.dump() {
				balance = append(balance, token.Balance)
			}
			if !slices.Equal(balance, tt.balance) {
				t.Errorf("NewTokenStream(%q) balance = %v, want %v", tt.source, balance, tt.balance)
			}
			for i, want := range tt.pairs {
				if got := ts.GetBlockTokenPairIndex(i); got != want {
					t.Errorf("NewTokenStream(%q).GetBlockTokenPairIndex(%d) = %d, want %d", tt.source, i, got, want)
				}
			}
		})
	}
}

func TestSkipUntilBalanced(t *testing.T) {
	const source = "a(b;c);d}e"
	for _, tt := range []struct {
		name        string
		skip, start int
		stop        StopFunc
		want        int
	}{
		{"stop before ;", 0, 0, stopAt(';', StopBefore), 5},
		{"stop after ;", 0, 0, stopAt(';', StopAfter), 6},
		{"end of the enclosing block", 2, 2, func(int) StopMode { return NoStop }, 4},
		{"nil stop", 2, 2, nil, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ts := NewTokenStream(source)
			ts.Skip(tt.skip)
			ts.SkipUntilBalanced(tt.start, tt.stop)
			if ts.TokenIndex != tt.want {
				t.Errorf("%s: SkipUntilBalanced(%d) TokenIndex = %d, want %d", tt.name, tt.start, ts.TokenIndex, tt.want)
			}
		})
	}
}

// stopAt returns a StopFunc that answers mode for code and NoStop otherwise.
func stopAt(code int, mode StopMode) StopFunc {
	return func(c int) StopMode {
		if c == code {
			return mode
		}
		return NoStop
	}
}

func TestSkipSC(t *testing.T) {
	ts := NewTokenStream(" /* a */ \n/**/b")
	ts.SkipSC()
	if ts.TokenType != Ident || ts.TokenStart != 14 {
		t.Errorf("SkipSC() stopped at %v %d, want %v 14", ts.TokenType, ts.TokenStart, Ident)
	}
	ts.SkipSC()
	if ts.TokenType != Ident {
		t.Errorf("SkipSC() on an ident moved to %v", ts.TokenType)
	}
}

func TestTokenStreamBOM(t *testing.T) {
	ts := NewTokenStream("\ufeffa b")
	want := []dumpToken{
		{Index: 0, Type: "ident-token", Chunk: "a", Balance: 3},
		{Index: 1, Type: "whitespace-token", Chunk: " ", Balance: 3},
		{Index: 2, Type: "ident-token", Chunk: "b", Balance: 3},
	}
	if got := ts.dump(); !slices.Equal(got, want) {
		t.Errorf("dump() = %v, want %v", got, want)
	}
	if got := ts.GetTokenStart(0); got != 3 {
		t.Errorf("GetTokenStart(0) = %d, want 3", got)
	}
}

func TestOffsetToLocationNewlines(t *testing.T) {
	for _, tt := range []struct {
		source string
		want   []Position
	}{
		{"\n\n\f\na\r", []Position{{0, 1, 1}, {1, 2, 1}, {2, 3, 1}, {3, 4, 1}, {4, 5, 1}, {5, 5, 2}, {6, 6, 1}}},
		{"\r\r\n\r", []Position{{0, 1, 1}, {1, 2, 1}, {2, 2, 2}, {3, 3, 1}, {4, 4, 1}}},
	} {
		t.Run(tt.source, func(t *testing.T) {
			l := &OffsetToLocation{Source: tt.source}
			for offset, want := range tt.want {
				if got := l.Position(offset); got != want {
					t.Errorf("OffsetToLocation{Source: %q}.Position(%d) = %+v, want %+v", tt.source, offset, got, want)
				}
			}
		})
	}
}

// A stream reuses its buffers across sources; nothing of a longer earlier
// source shows through a shorter later one, and a longer one grows them.
func TestTokenStreamReuseBuffers(t *testing.T) {
	ts := NewTokenStream("f([{a}]) (")
	for _, source := range []string{"a", ")", strings.Repeat(",", 100)} {
		ts.SetSource(source)
		want := NewTokenStream(source)
		if !slices.Equal(ts.dump(), want.dump()) {
			t.Errorf("after SetSource(%q) dump() = %v, want %v", source, ts.dump(), want.dump())
		}
		if !slices.Equal(ts.Balance, want.Balance) {
			t.Errorf("after SetSource(%q) Balance = %v, want %v", source, ts.Balance, want.Balance)
		}
		if got := ts.GetTokenType(ts.TokenCount); got != EOF {
			t.Errorf("after SetSource(%q) GetTokenType(%d) = %v, want %v", source, ts.TokenCount, got, EOF)
		}
		if got := ts.GetTokenEnd(ts.TokenCount - 1); got != len(source) {
			t.Errorf("after SetSource(%q) GetTokenEnd(%d) = %d, want %d", source, ts.TokenCount-1, got, len(source))
		}
	}
}

// Offsets around a source with no tokens or only a BOM. Upstream counts the
// BOM as one code unit, so its offsets here are 2 less.
func TestTokenStreamEmptyAndBOM(t *testing.T) {
	empty := NewTokenStream("")
	if got := empty.GetTokenStart(-1); got != 0 {
		t.Errorf(`NewTokenStream("").GetTokenStart(-1) = %d, want 0`, got)
	}
	if got := empty.LookupOffset(0); got != 0 {
		t.Errorf(`NewTokenStream("").LookupOffset(0) = %d, want 0`, got)
	}
	if empty.LookupValue(0, "") {
		t.Errorf(`NewTokenStream("").LookupValue(0, "") = true, want false`)
	}

	bomOnly := NewTokenStream("\ufeff")
	if got := bomOnly.LookupOffset(0); got != 3 {
		t.Errorf(`NewTokenStream("\ufeff").LookupOffset(0) = %d, want 3`, got)
	}
	if bomOnly.LookupValue(0, "") {
		t.Errorf(`NewTokenStream("\ufeff").LookupValue(0, "") = true, want false`)
	}

	bom := NewTokenStream("\ufeffa b")
	bom.Skip(2)
	for _, tt := range []struct {
		name      string
		got, want int
	}{
		{"GetTokenStart(0)", bom.GetTokenStart(0), 3},
		{"GetTokenStart(1)", bom.GetTokenStart(1), 4},
		{"LookupOffset(0)", bom.LookupOffset(0), 5},
		{"LookupOffset(1)", bom.LookupOffset(1), 6},
	} {
		if tt.got != tt.want {
			t.Errorf(`NewTokenStream("\ufeffa b") at 2: %s = %d, want %d`, tt.name, tt.got, tt.want)
		}
	}
	if bom.LookupValue(1, "") {
		t.Errorf(`at EOF LookupValue(1, "") = true, want false`)
	}
	if !bom.LookupValue(0, "b") {
		t.Errorf(`LookupValue(0, "b") = false, want true`)
	}
}

func TestOffsetToLocationLines(t *testing.T) {
	const source = "a\r\nb\nc\rd\fe"
	l := &OffsetToLocation{Source: source, Filename: "f.css", StartOffset: 10, StartLine: 3, StartColumn: 5}
	want := []Position{{10, 3, 5}, {11, 3, 6}, {12, 3, 7}, {13, 4, 1}, {14, 4, 2}, {15, 5, 1}, {16, 5, 2}, {17, 6, 1}, {18, 6, 2}, {19, 7, 1}, {20, 7, 2}}
	for offset, want := range want {
		if got := l.Position(offset); got != want {
			t.Errorf("Position(%d) = %+v, want %+v", offset, got, want)
		}
	}
	wantRange := Location{Filename: "f.css", Start: want[1], End: want[4]}
	if got := l.Range(1, 4); got != wantRange {
		t.Errorf("Range(1, 4) = %+v, want %+v", got, wantRange)
	}

	// upstream offsets 1..4 after a one code unit BOM are 3..6 here; the
	// BOM itself is on line 0, column 0, which upstream leaves unset
	bom := &OffsetToLocation{Source: "\ufeffa\nb"}
	for offset, want := range []Position{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {3, 1, 1}, {4, 1, 2}, {5, 2, 1}, {6, 2, 2}} {
		if got := bom.Position(offset); got != want {
			t.Errorf("Position(%d) = %+v, want %+v", offset, got, want)
		}
	}
}

// Offsets out of range are clamped to the source, and the zero value
// converts offsets in an empty source.
func TestOffsetToLocationClamp(t *testing.T) {
	l := &OffsetToLocation{Source: "ab\nc", StartOffset: 7}
	for _, tt := range []struct {
		offset int
		want   Position
	}{
		{-5, Position{7, 1, 1}},
		{100, Position{11, 2, 2}},
	} {
		t.Run(fmt.Sprint(tt.offset), func(t *testing.T) {
			if got := l.Position(tt.offset); got != tt.want {
				t.Errorf("Position(%d) = %+v, want %+v", tt.offset, got, tt.want)
			}
		})
	}

	var zero OffsetToLocation
	if got, want := zero.Range(-1, 1), (Location{Start: Position{0, 1, 1}, End: Position{0, 1, 1}}); got != want {
		t.Errorf("zero OffsetToLocation Range(-1, 1) = %+v, want %+v", got, want)
	}
}

func TestTokenTypeString(t *testing.T) {
	for _, tt := range []struct {
		tokenType TokenType
		want      string
	}{
		{EOF, "EOF-token"}, {Comment, "comment-token"}, {LeftCurlyBracket, "{-token"}, {26, "TokenType(26)"}, {-1, "TokenType(-1)"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.tokenType.String(); got != tt.want {
				t.Errorf("TokenType(%d).String() = %q, want %q", int(tt.tokenType), got, tt.want)
			}
		})
	}
}

// Block predicates answer false for types outside the defined range.
func TestTokenTypeBlockOutOfRange(t *testing.T) {
	for _, tokenType := range []TokenType{-1, Comment + 1, 32, 1 << 20} {
		if tokenType.IsBlockOpener() || tokenType.IsBlockCloser() {
			t.Errorf("TokenType(%d) IsBlockOpener() = %t, IsBlockCloser() = %t, want false, false", int(tokenType), tokenType.IsBlockOpener(), tokenType.IsBlockCloser())
		}
		if got := tokenType.closer(); got != EOF {
			t.Errorf("TokenType(%d).closer() = %v, want %v", int(tokenType), got, EOF)
		}
	}
}

// The zero TokenStream answers reads as a stream over "" does instead of
// indexing missing buffers.
func TestTokenStreamZeroValue(t *testing.T) {
	var zero TokenStream
	empty := NewTokenStream("")
	for _, index := range []int{-2, -1, 0, 1, 5} {
		for _, read := range []struct {
			name string
			fn   func(ts *TokenStream) any
		}{
			{"GetTokenType", func(ts *TokenStream) any { return ts.GetTokenType(index) }},
			{"GetTokenStart", func(ts *TokenStream) any { return ts.GetTokenStart(index) }},
			{"GetTokenEnd", func(ts *TokenStream) any { return ts.GetTokenEnd(index) }},
			{"GetBlockTokenPairIndex", func(ts *TokenStream) any { return ts.GetBlockTokenPairIndex(index) }},
			{"LookupType", func(ts *TokenStream) any { return ts.LookupType(index) }},
			{"LookupTypeNonSC", func(ts *TokenStream) any { return ts.LookupTypeNonSC(index) }},
			{"LookupOffset", func(ts *TokenStream) any { return ts.LookupOffset(index) }},
			{"LookupOffsetNonSC", func(ts *TokenStream) any { return ts.LookupOffsetNonSC(index) }},
			{"LookupValue", func(ts *TokenStream) any { return ts.LookupValue(index, "") }},
			{"IsBalanceEdge", func(ts *TokenStream) any { return ts.IsBalanceEdge(index) }},
			{"IsDelim", func(ts *TokenStream) any { return ts.IsDelim('.', index) }},
		} {
			if got, want := read.fn(&zero), read.fn(empty); got != want {
				t.Errorf("zero TokenStream %s(%d) = %v, want %v", read.name, index, got, want)
			}
		}
	}
	if got := zero.dump(); len(got) != 0 {
		t.Errorf("zero TokenStream dump() = %v, want no tokens", got)
	}
	zero.SkipUntilBalanced(-1, nil)
	if !zero.EOF || zero.TokenType != EOF || zero.TokenIndex != 0 {
		t.Errorf("after SkipUntilBalanced EOF, TokenType, TokenIndex = %t, %v, %d, want true, %v, 0", zero.EOF, zero.TokenType, zero.TokenIndex, EOF)
	}
}

// A negative start token is the first token.
func TestSkipUntilBalancedNegativeStart(t *testing.T) {
	ts := NewTokenStream("a;b")
	ts.SkipUntilBalanced(-1, stopAt(';', StopBefore))
	if ts.TokenIndex != 1 {
		t.Errorf("SkipUntilBalanced(-1) TokenIndex = %d, want 1", ts.TokenIndex)
	}
}

// A negative code has no category instead of indexing before the table.
func TestCharCodeCategoryNegative(t *testing.T) {
	if got := charCodeCategory(-1); got != -1 {
		t.Errorf("charCodeCategory(-1) = %d, want -1", got)
	}
}
