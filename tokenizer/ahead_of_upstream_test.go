// Go only: tests of behavior that upstream v3.2.1 does not have.
// `go test -run TestAhead ./...` runs them in every package.

package tokenizer

import (
	"slices"
	"strings"
	"testing"
)

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
