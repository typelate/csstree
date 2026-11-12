// Go only: tests of behavior that upstream v3.2.1 does not have.
// `go test -run TestAhead ./...` runs them in every package. Their AST
// fixtures, in upstream's fixture format, are in testdata/ahead-of-upstream.

package csstree

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/typelate/csstree/tokenizer"
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

// A functional pseudo whose argument is only whitespace or a comment has no
// children.
func TestAheadEmptyFunctionalPseudo(t *testing.T) {
	runAheadASTFixtures(t,
		"selector/functional-pseudo/is.json",
		"selector/functional-pseudo/not.json",
		"selector/functional-pseudo/slotted.json",
	)
}

// firstValueNode parses source as a value and returns it with its first
// child.
func firstValueNode(t *testing.T, source string) (*Value, Node) {
	t.Helper()
	ast, err := Parse(source, ParseOptions{Context: ContextValue})
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", source, err)
	}
	value, ok := ast.(*Value)
	if !ok {
		t.Fatalf("Parse(%q) is a %T, want a *Value", source, ast)
	}
	return value, value.Children.First()
}

// A token that ends with a \ escaping the end of its source, such as the
// ident `c\`, is generated with a newline after it, so that the \ does not
// escape what the generator writes next.
func TestAheadGenerateEscapeBeforeEOF(t *testing.T) {
	for _, source := range []string{`c\`, `#c\`, `1c\`, `c\\\`, `-\`, `\`} {
		t.Run("in a value: "+source, func(t *testing.T) {
			_, first := firstValueNode(t, source)
			value := &Value{Children: new(List[Node]).AppendData(first).AppendData(&Identifier{Name: "z"})}

			// the value context does not take the \ delim the newline leaves,
			// so look at the tokens
			generated := mustGenerate(t, value)
			var last string
			tokenizer.Tokenize(generated, func(tokenType tokenizer.TokenType, start, end int) {
				last = tokenType.String() + " " + generated[start:end]
			})
			if want := "ident-token z"; last != want {
				t.Errorf("Generate(Parse(%q) and the identifier z) = %q, whose last token is %q, want %q", source, generated, last, want)
			}
		})
	}

	for _, source := range []string{`a{b:c\`, `a{b:#c\`, `a{b:1c\`, `a{--b:c\`, `a{b\`, `@a b\`, `@a{b:c\`, `a{b:c\\\`} {
		t.Run("before the end of a block: "+source, func(t *testing.T) {
			generated := mustGenerate(t, mustParse(t, source, ParseOptions{}))

			// a rule after the generated text stays a rule of the style sheet
			sheet, ok := mustParse(t, generated+"\nz{}", ParseOptions{}).(*StyleSheet)
			if !ok {
				t.Fatalf("Parse(%q) is not a *StyleSheet", generated+"\nz{}")
			}
			rule, ok := sheet.Children.Last().(*Rule)
			if !ok || sheet.Children.Len() != 2 || mustGenerate(t, rule) != "z{}" {
				t.Errorf("Parse(%q), generated from Parse(%q) and followed by z{}, = %s, want two children, the last one z{}",
					generated, source, jsonText(t, toPlainJSON(t, sheet), ""))
			}

			if second := mustGenerate(t, mustParse(t, generated, ParseOptions{})); second != generated {
				t.Errorf("Generate(Parse(%q)) = %q, want the same", generated, second)
			}
		})
	}
}

// A name is "url" by its value, after escapes (CSS Syntax 3, 4.3.4). The
// tokenizer package has the token tests.
func TestAheadEscapedURLNames(t *testing.T) {
	names := []string{`\75 Rl`, `u\72l`, `ur\6c`, `\u\r\l`, `\000075\000052\00006c`, "\\75\r\nr\\6C"}

	for _, name := range names {
		t.Run("decodes the URL value after "+name, func(t *testing.T) {
			source := name + `(foo\ bar.png)`
			if got, want := DecodeURL(source), "foo bar.png"; got != want {
				t.Errorf("DecodeURL(%q) = %q, want %q", source, got, want)
			}
		})

		for _, suffix := range []string{`(foo)`, `( "foo" )`, `( 'foo' )`} {
			source := name + suffix
			t.Run("parses "+source+" as a URL", func(t *testing.T) {
				ast, first := firstValueNode(t, source)
				url, ok := first.(*URL)
				if !ok {
					t.Fatalf("Parse(%q) first child is a %T, want a *URL", source, first)
				}
				if url.Value != "foo" {
					t.Errorf("Parse(%q) first child value = %q, want %q", source, url.Value, "foo")
				}
				generated, err := Generate(ast, GenerateOptions{})
				if err != nil {
					t.Fatalf("Generate(Parse(%q)) error = %v", source, err)
				}
				if generated != "url(foo)" {
					t.Errorf("Generate(Parse(%q)) = %q, want %q", source, generated, "url(foo)")
				}
			})
		}
	}

	for _, name := range []string{"curl", "urls", "urlx", `\75rlx`, `u\0072lx`, `\000075xrl`} {
		t.Run("keeps the distinct function name "+name, func(t *testing.T) {
			source := name + "(foo)"
			_, first := firstValueNode(t, source)
			if _, ok := first.(*Function); !ok {
				t.Errorf("Parse(%q) first child is a %T, want a *Function", source, first)
			}
		})
	}
}

// Escaped whitespace at the end of a url() is part of its value, and the
// value survives generating and parsing again.
func TestAheadEscapedURLEndings(t *testing.T) {
	tests := []decodeEncodeTest{
		{`url(data:x\ )`, `data:x `},
		{`url(a\  )`, `a `},
		{`url(\ )`, ` `},
		{"url(a\\\t )", "a\t"},
		{`url(a\\ )`, `a\`},
		{`url(a\\\ )`, `a\ `},
		{`url(a\20  )`, `a `},
		{`url(a\)`, `a)`},
		{`url(data:x\ `, `data:x `},
	}

	forEachDecodeEncodeTest(t, tests, DecodeURL)

	for _, test := range tests {
		t.Run("preserves the parsed value through generation: "+jsonString(test.from), func(t *testing.T) {
			ast, first := firstValueNode(t, test.from)
			url, ok := first.(*URL)
			if !ok {
				t.Fatalf("Parse(%q) first child is a %T, want a *URL", test.from, first)
			}
			if url.Value != test.to {
				t.Errorf("Parse(%q) first child value = %q, want %q", test.from, url.Value, test.to)
			}

			generated, err := Generate(ast, GenerateOptions{})
			if err != nil {
				t.Fatalf("Generate(Parse(%q)) error = %v", test.from, err)
			}
			_, first = firstValueNode(t, generated)
			url, ok = first.(*URL)
			if !ok {
				t.Fatalf("Parse(%q) first child is a %T, want a *URL", generated, first)
			}
			if url.Value != test.to {
				t.Errorf("Parse(Generate(Parse(%q))) = Parse(%q) first child value = %q, want %q", test.from, generated, url.Value, test.to)
			}
		})
	}
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
