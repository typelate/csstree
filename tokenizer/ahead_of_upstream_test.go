// Go only: tests of behavior that upstream v3.2.1 does not have.
// `go test -run TestAhead ./...` runs them in every package.

package tokenizer

import (
	"slices"
	"strings"
	"testing"
)

type typedChunk struct {
	Type  TokenType
	Value string
}

func typedChunks(source string) []typedChunk {
	var result []typedChunk
	Tokenize(source, func(tokenType TokenType, start, end int) {
		result = append(result, typedChunk{tokenType, source[start:end]})
	})
	return result
}

// A backslash before EOF is a valid escape (CSS Syntax 3, 4.3.8), so it
// starts an ident.
func TestAheadEscapeBeforeEOF(t *testing.T) {
	const source = `\`
	want := []typedChunk{{Ident, source}}
	if got := typedChunks(source); !slices.Equal(got, want) {
		t.Errorf("Tokenize(%q) = %v, want %v", source, got, want)
	}
}

// A name is "url" by its value, after escapes (CSS Syntax 3, 4.3.4).
func TestAheadEscapedURLNames(t *testing.T) {
	names := []string{`\75 Rl`, `u\72l`, `ur\6c`, `\u\r\l`, `\000075\000052\00006c`, "\\75\r\nr\\6C"}

	for _, name := range names {
		t.Run("tokenizes an unquoted URL named "+name, func(t *testing.T) {
			source := name + `(foo\ bar.png)`
			want := []typedChunk{{URL, source}}
			if got := typedChunks(source); !slices.Equal(got, want) {
				t.Errorf("Tokenize(%q) = %v, want %v", source, got, want)
			}
		})

		t.Run("preserves the quoted function boundary for "+name, func(t *testing.T) {
			source := name + `( "foo" )`
			want := []typedChunk{
				{Function, name + "("},
				{WhiteSpace, " "},
				{String, `"foo"`},
				{WhiteSpace, " "},
				{RightParenthesis, ")"},
			}
			if got := typedChunks(source); !slices.Equal(got, want) {
				t.Errorf("Tokenize(%q) = %v, want %v", source, got, want)
			}
		})
	}

	for _, name := range []string{"curl", "urls", "urlx", `\75rlx`, `u\0072lx`, `\000075xrl`} {
		t.Run("keeps the distinct function name "+name, func(t *testing.T) {
			source := name + "(foo)"
			got := typedChunks(source)
			if len(got) == 0 || got[0].Type != Function {
				t.Errorf("Tokenize(%q) = %v, want a first token of type %v", source, got, Function)
			}
		})
	}

	t.Run("keeps bad URL handling for an escaped name", func(t *testing.T) {
		const source = `\75rl(a b)`
		want := []typedChunk{{BadURL, source}}
		if got := typedChunks(source); !slices.Equal(got, want) {
			t.Errorf("Tokenize(%q) = %v, want %v", source, got, want)
		}
	})
}

// A stream reused for a smaller source gives the tokens of a fresh stream.
func TestAheadSetSourceAfterLargerSource(t *testing.T) {
	const css = ".test\n{\n  prop: url(foo/bar.jpg) url( a\\(\\33 \\).\\ \\\"\\'test ) calc(1 + 1) \\x \\aa ;\n}<!--<-->\\\n"

	stream := NewTokenStream(strings.Repeat("a(", 10000) + css)
	stream.SetSource(css)

	want := NewTokenStream(css).dump()
	if got := stream.dump(); !slices.Equal(got, want) {
		t.Errorf("dump() after SetSource = %v, want %v", got, want)
	}
}

// A stream reused for a smaller source gives the balance of a fresh stream,
// here for a source with an unmatched "}".
func TestAheadSetSourceBalanceAfterLargerSource(t *testing.T) {
	const source = "[x]{}a{b:c}}@supports not (x:y){a{b:c}}"

	stream := NewTokenStream("/**/ " + strings.Repeat("a{}", 100))
	stream.SetSource(source)
	fresh := NewTokenStream(source)

	got := stream.Balance[:stream.TokenCount]
	want := fresh.Balance[:fresh.TokenCount]
	if !slices.Equal(got, want) {
		t.Errorf("Balance after SetSource = %v, want %v", got, want)
	}
}
