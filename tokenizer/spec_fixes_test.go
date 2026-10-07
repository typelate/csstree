package tokenizer

// Places where the port follows CSS Syntax Module Level 3 instead of
// upstream csstree 3.2.1. Each case name gives the section and, after
// "upstream:", what csstree produces for the same input.

import (
	"slices"
	"testing"
)

func TestTokenizeSpecFixes(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
		want   []tokenChunk
	}{
		// §4.3.5 consume a string token: on a newline, "Reconsume the
		// current input code point, create a <bad-string-token>, and return it."
		{
			`§4.3.5 a bad-string ends before LF (upstream: "a\n bad-string)`,
			"\"a\nb\"",
			[]tokenChunk{{"bad-string-token", "\"a"}, {"whitespace-token", "\n"}, {"ident-token", "b"}, {"string-token", "\""}},
		},
		{
			`§4.3.5 a bad-string ends before CR (upstream: "a\r bad-string)`,
			"\"a\rb\"",
			[]tokenChunk{{"bad-string-token", "\"a"}, {"whitespace-token", "\r"}, {"ident-token", "b"}, {"string-token", "\""}},
		},
		{
			`§4.3.5 a bad-string ends before CRLF (upstream: "a\r\n bad-string)`,
			"\"a\r\nb\"",
			[]tokenChunk{{"bad-string-token", "\"a"}, {"whitespace-token", "\r\n"}, {"ident-token", "b"}, {"string-token", "\""}},
		},
		{
			`§4.3.5 a bad-string ends before FF (upstream: 'a\f bad-string)`,
			"'a\fb",
			[]tokenChunk{{"bad-string-token", "'a"}, {"whitespace-token", "\f"}, {"ident-token", "b"}},
		},
		{
			`§4.3.5 a newline right after the quote (upstream: "\n bad-string)`,
			"\"\n",
			[]tokenChunk{{"bad-string-token", "\""}, {"whitespace-token", "\n"}},
		},
		// §4.3.14 consume the remnants of a bad url: after an escape the
		// loop goes on with the code point right after it.
		{
			`§4.3.14 the ) after an escape ends the remnants (upstream: bad-url url(a"\a)b), ident c)`,
			"url(a\"\\a)b)c",
			[]tokenChunk{{"bad-url-token", "url(a\"\\a)"}, {"ident-token", "b"}, {")-token", ")"}, {"ident-token", "c"}},
		},
		{
			`§4.3.14 an escaped ) then ) (upstream: bad-url runs to EOF)`,
			"url(x\"\\))d",
			[]tokenChunk{{"bad-url-token", "url(x\"\\))"}, {"ident-token", "d"}},
		},
		{
			`§4.3.14 a hex escape and its whitespace (upstream: bad-url url(x"\31 )), ident d)`,
			"url(x\"\\31 ))d",
			[]tokenChunk{{"bad-url-token", "url(x\"\\31 )"}, {")-token", ")"}, {"ident-token", "d"}},
		},
		{
			`§4.3.14 an escape at EOF`,
			"url(x\"\\a",
			[]tokenChunk{{"bad-url-token", "url(x\"\\a"}},
		},
		{
			`§4.3.14 a backslash at EOF`,
			"url(x\"\\",
			[]tokenChunk{{"bad-url-token", "url(x\"\\"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenizeChunks(tt.source); !slices.Equal(got, tt.want) {
				t.Errorf("Tokenize(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}
