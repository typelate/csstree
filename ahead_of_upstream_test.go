// Go only: tests of behavior that upstream v3.2.1 does not have.
// `go test -run TestAhead ./...` runs them in every package. Their AST
// fixtures, in upstream's fixture format, are in testdata/ahead-of-upstream.

package csstree

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

const aheadFixtureDir = "testdata/ahead-of-upstream"

// runAheadASTFixtures runs the parse and generate fixture tests over the
// fixture files, each named "<scope>/<path>.json" under aheadFixtureDir.
func runAheadASTFixtures(t *testing.T, names ...string) {
	t.Helper()

	for _, name := range names {
		scope, _, _ := strings.Cut(name, "/")
		file, err := readASTFixtureFile(filepath.Join(aheadFixtureDir, filepath.FromSlash(name)), scope)
		if err != nil {
			t.Fatalf("reading fixture %s: %v", name, err)
		}

		for _, test := range file.Tests {
			t.Run(test.Name, func(t *testing.T) {
				t.Run("parse", func(t *testing.T) {
					ast, err := Parse(test.Source, test.ParseOptions(false))
					if err != nil {
						t.Fatalf("Parse(%q) error = %v", test.Source, err)
					}
					assertASTEqualNoLoc(t, test.Source, ast, test.AST)
				})
				t.Run("generate", func(t *testing.T) {
					createGenerateTests(t, test)
				})
			})
		}
	}
}

// Nested style rules that start with a class, ID, attribute, universal or
// pseudo-class selector.
func TestAheadNestedRules(t *testing.T) {
	runAheadASTFixtures(t, "rule/nesting.json")
}

// A url() of only whitespace decodes to the empty string.
func TestAheadDecodeWhitespaceOnlyURL(t *testing.T) {
	forEachDecodeEncodeTest(t, []decodeEncodeTest{
		{`url( )`, ``},
		{`url(  )`, ``},
		{"url(\t)", ``},
		{"url( \t )", ``},
	}, DecodeURL)
}

// The EOF position of sources around 16 KiB, parsed by a syntax that has
// parsed a smaller source before.
func TestAheadEOFPositionsAfterSmallerSource(t *testing.T) {
	syntax := mustFork(t, &Config{})

	if _, err := syntax.Parse("a{}", ParseOptions{Positions: true}); err != nil {
		t.Fatalf(`Parse("a{}") error = %v`, err)
	}

	for _, length := range []int{16383, 16384, 16385} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			source := "a{" + strings.Repeat(" ", length-3) + "}"
			ast, err := syntax.Parse(source, ParseOptions{Positions: true})
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			sheet, ok := ast.(*StyleSheet)
			if !ok || sheet.Loc == nil {
				t.Fatalf("Parse() = %T, want a *StyleSheet with a location", ast)
			}
			want := Location{}.End
			want.Offset, want.Line, want.Column = length, 1, length+1
			if sheet.Loc.End != want {
				t.Errorf("StyleSheet end = %+v, want %+v", sheet.Loc.End, want)
			}

			rule, ok := sheet.Children.First().(*Rule)
			if !ok || rule.Block == nil || rule.Block.Loc == nil {
				t.Fatalf("first child = %T, want a *Rule whose block has a location", sheet.Children.First())
			}
			if rule.Block.Loc.End != want {
				t.Errorf("Block end = %+v, want %+v", rule.Block.Loc.End, want)
			}
		})
	}
}

// A source with an unmatched "}" parses after a larger source has been
// parsed.
func TestAheadParseRecoveryAfterLargerSource(t *testing.T) {
	options := ParseOptions{
		NoParseAtrulePrelude: true,
		NoParseValue:         true,
	}

	larger := "/**/ " + strings.Repeat("a{}", 100)
	if _, err := Parse(larger, options); err != nil {
		t.Fatalf("Parse(larger source) error = %v", err)
	}

	const source = "[x]{}a{b:c}}@supports not (x:y){a{b:c}}"
	if _, err := Parse(source, options); err != nil {
		t.Errorf("Parse(%q) error = %v", source, err)
	}
}
