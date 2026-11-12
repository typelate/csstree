// Go only: tests of behavior that upstream v3.2.1 does not have.
// `go test -run TestAhead ./...` runs them in every package. Their AST
// fixtures, in upstream's fixture format, are in testdata/ahead-of-upstream.

package csstree

import (
	"errors"
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

// A value with env() that does not match as written is not a mismatch: what
// env() stands for is unknown.
func TestAheadEnvPassthrough(t *testing.T) {
	lexer, err := DefaultSyntax().NewLexer(&Config{
		Types: map[string]string{"bar": "foo"},
	})
	if err != nil {
		t.Fatalf("NewLexer() error = %v", err)
	}

	for _, value := range []string{
		"env(name)",
		"env(name) qux",
		"ENV(name)",
		"ENV(name) qux",
		`\65nv(name)`,
		"foo env(name)",
		"rgb(env(name))",
	} {
		t.Run(value, func(t *testing.T) {
			match := outcome(lexer.MatchString("<bar>", value))
			if match.Matched != nil {
				t.Errorf("MatchString(%q, %q) matched, want no match", "<bar>", value)
			}
			if !errors.Is(match.err, ErrEnvNotSupported) {
				t.Errorf("MatchString(%q, %q) error = %v, want %v", "<bar>", value, match.err, ErrEnvNotSupported)
			}
		})
	}

	t.Run("a value without env() still mismatches", func(t *testing.T) {
		assertSyntaxMatchError(t, `MatchString("<bar>", "qux")`, outcome(lexer.MatchString("<bar>", "qux")))
	})

	t.Run("a value that matches with env() still matches", func(t *testing.T) {
		assertMatchNoError(t, `MatchString("<bar>", "foo")`, outcome(lexer.MatchString("<bar>", "foo")))
		const value = "calc(1px + env(safe-area-inset-top))"
		assertMatchNoError(t, fmt.Sprintf("MatchPropertyString(%q, %q)", "width", value),
			outcome(DefaultLexer().MatchPropertyString("width", value)))
	})

	t.Run("var() comes first", func(t *testing.T) {
		_, err := lexer.MatchString("<bar>", "env(name) var(--x)")
		if !errors.Is(err, ErrVarNotSupported) {
			t.Errorf("MatchString() error = %v, want %v", err, ErrVarNotSupported)
		}
	})
}

// Nested style rules that start with a class, ID, attribute, universal or
// pseudo-class selector.
func TestAheadNestedRules(t *testing.T) {
	runAheadASTFixtures(t, "rule/nesting.json")
}

// An attribute selector left open at EOF is closed there.
func TestAheadUnclosedAttributeSelector(t *testing.T) {
	runAheadASTFixtures(t, "selector/AttributeSelector.json")
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

// Relative colors (CSS Color 5): `from <color>` and channel keywords in the
// color functions.
func TestAheadRelativeColorSyntax(t *testing.T) {
	lexer := DefaultLexer()

	for _, value := range []string{
		"rgb(from red r g b)",
		"rgb(from #123456 r g b / 0.5)",
		"rgb(from blue r g b / alpha)",
		"rgba(from red r g b / 0.5)",
		"hsl(from red h s l)",
		"hsl(from red h s l / 0.5)",
		"hsla(from red h s l / alpha)",
		"hwb(from red h w b)",
		"hwb(from red h w b / 0.5)",
		"lab(from red l a b)",
		"lab(from red l a b / 0.5)",
		"lch(from red l c h)",
		"lch(from red l c h / 0.5)",
		"oklab(from red l a b)",
		"oklab(from red l a b / 0.5)",
		"oklch(from #0af l c h)",
		"oklch(from red l c h / alpha)",
		"oklch(from red calc(l * 0.9) c h)",
		"color(from red srgb r g b)",
		"color(from red display-p3 r g b / alpha)",
		"color(from red xyz-d50 x y z / 0.5)",
	} {
		t.Run(value, func(t *testing.T) {
			assertMatchNoError(t, fmt.Sprintf("MatchPropertyString(%q, %q)", "color", value),
				outcome(lexer.MatchPropertyString("color", value)))
		})
	}

	// more of the same, and the keywords of each color space of color()
	for _, value := range []string{
		"RGB(FROM red R G B / ALPHA)",
		"rgb(from rgb(from red r g b) b g r)",
		"rgb(from red calc(r + 10) min(g, b) calc((b + alpha) / 2))",
		"hsl(from red calc(h + 180) s l / calc(alpha * 0.5))",
		"lab(from red l a b / calc(alpha - 0.1))",
		"color(from red xyz calc(x * 2) y z)",
		"color(from color(xyz 0 0 0) srgb calc(r * 2) g b)",
		"rgb(from red 10 20% none)",
	} {
		t.Run(value, func(t *testing.T) {
			assertMatchNoError(t, fmt.Sprintf("MatchPropertyString(%q, %q)", "color", value),
				outcome(lexer.MatchPropertyString("color", value)))
		})
	}

	for _, value := range []string{
		"rgb(r g b)",                             // channel keywords need an origin color
		"rgb(0 calc(g) 0)",                       // also inside a calculation
		"rgb(from red h s l)",                    // the keywords of another function
		"rgb(from red calc(l) g b)",              // also inside a calculation
		"rgb(from red, r, g, b)",                 // no legacy syntax with an origin color
		"rgb(from r g b)",                        // an origin color is required
		"color(from red srgb x y z)",             // the keywords of another color space
		"color(from red srgb calc(x) g b)",       // also inside a calculation
		"color(from red xyz calc(r) y z)",        // also inside a calculation
		"rgb(from red r g b) calc(r)",            // after the color function
		"rgb(from red r g b / calc(1px))",        // a calculation is still type checked
		"rgb(from red r g calc(b * 1px))",        // a channel keyword is a number
		"rgb(from hsl(from red h s l) h s l)",    // an inner function's keywords
		"rgb(from red r g b / alpha alpha)",      // one alpha
		"oklch(from red l c)",                    // three channels
		"linear-gradient(from red, calc(r))",     // not a color function
		"rgb(from red r g rgb(0 0 calc(b)))",     // not relative itself
		"rgb(from red var-like(calc(r)) g b)",    // inside an unknown function
		"rgb(from red r g b / calc(infinity x))", // a broken calculation
	} {
		t.Run("invalid "+value, func(t *testing.T) {
			assertSyntaxMatchError(t, fmt.Sprintf("MatchPropertyString(%q, %q)", "color", value),
				outcome(lexer.MatchPropertyString("color", value)))
		})
	}
}
