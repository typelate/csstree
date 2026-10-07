// Ported from lib/__tests/exports.js.
//
// Upstream checks the keys of each package entry point. Go packages have no
// runtime list of exports, so the name checks reference each exported Go
// counterpart: they fail to compile when a name goes missing.

package csstree

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/typelate/csstree/definitionsyntax"
	"github.com/typelate/csstree/tokenizer"
)

func TestExports(t *testing.T) {
	const fixtureFilename = "testdata/stringify.css"
	cssData, err := os.ReadFile(fixtureFilename)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", fixtureFilename, err)
	}
	css := string(cssData)
	expectedData, err := os.ReadFile(strings.Replace(fixtureFilename, ".css", ".ast", 1))
	if err != nil {
		t.Fatalf("os.ReadFile(...): %v", err)
	}
	var expectedPlain map[string]any
	if err := json.Unmarshal(expectedData, &expectedPlain); err != nil {
		t.Fatalf("json.Unmarshal(expectedData, &expectedPlain): %v", err)
	}
	expectedAST, err := FromPlainObject(expectedPlain)
	if err != nil {
		t.Fatalf("FromPlainObject(expectedPlain): %v", err)
	}

	t.Run("exports / entry points", func(t *testing.T) {
		t.Run("tokenizer", func(t *testing.T) {
			// upstream: the keys of css-tree/tokenizer, as Go names; the
			// tokenTypes namespace object has no counterpart (the token
			// types are constants of package tokenizer), and the char code
			// helpers only the tokenizer uses (charCodeCategory, isBOM,
			// consumeName, ...) and tokenNames are unexported
			_ = []any{
				tokenizer.AtKeyword,
				tokenizer.BadString,
				tokenizer.BadURL,
				tokenizer.CDC,
				tokenizer.CDO,
				tokenizer.Colon,
				tokenizer.Comma,
				tokenizer.Comment,
				tokenizer.Delim,
				tokenizer.Dimension,
				tokenizer.EOF,
				tokenizer.Function,
				tokenizer.Hash,
				tokenizer.Ident,
				tokenizer.LeftCurlyBracket,
				tokenizer.LeftParenthesis,
				tokenizer.LeftSquareBracket,
				tokenizer.Number,
				(*tokenizer.OffsetToLocation).Position,
				tokenizer.Percentage,
				tokenizer.RightCurlyBracket,
				tokenizer.RightParenthesis,
				tokenizer.RightSquareBracket,
				tokenizer.Semicolon,
				tokenizer.String,
				tokenizer.NewTokenStream,
				tokenizer.URL,
				tokenizer.WhiteSpace,
				tokenizer.CmpChar,
				tokenizer.CmpStr,
				tokenizer.ConsumeEscaped,
				tokenizer.ConsumeNumber,
				tokenizer.DecodeEscaped,
				tokenizer.FindWhiteSpaceStart,
				tokenizer.IsDigit,
				tokenizer.IsHexDigit,
				tokenizer.IsIdentifierStart,
				tokenizer.IsName,
				tokenizer.IsValidEscape,
				tokenizer.IsWhiteSpace,
				tokenizer.Tokenize,
			}
		})

		t.Run("parser", func(t *testing.T) {
			ast, err := Parse(css, ParseOptions{})
			if err != nil {
				t.Fatalf("Parse(%q, ParseOptions{}): %v", css, err)
			}
			assertJSONEqual(t, "Parse(stringify.css)", stripLoc(toPlainJSON(t, ast)), stripLoc(any(expectedPlain)))
		})

		t.Run("selector-parser", func(t *testing.T) {
			type selectorTest struct {
				css      string
				context  string
				expected Node
			}
			var selectors []selectorTest

			// collect all the selectors
			if err := Walk(expectedAST, WalkOptions{
				Enter: func(_ *WalkContext, node Node, _ *ListItem[Node], _ *List[Node]) WalkAction {
					if node.Type() == "SelectorList" || node.Type() == "Selector" {
						css, err := Generate(node, GenerateOptions{})
						if err != nil {
							t.Fatalf("Generate(node, GenerateOptions{}): %v", err)
						}
						context := "selector"
						if node.Type() == "SelectorList" {
							context = "selectorList"
						}
						selectors = append(selectors, selectorTest{
							css:      css,
							context:  context,
							expected: node,
						})

						return WalkSkip
					}
					return WalkContinue
				},
			}); err != nil {
				t.Fatalf("Walk(expectedAST, ...): %v", err)
			}

			for _, test := range selectors {
				t.Run(test.css, func(t *testing.T) {
					ast, err := ParseSelector(test.css, ParseOptions{Context: test.context})
					if err != nil {
						t.Fatalf("ParseSelector(%q, ...): %v", test.css, err)
					}
					assertASTEqualNoLoc(t, test.css, ast, test.expected)
				})
			}
		})

		t.Run("generator", func(t *testing.T) {
			generate := CreateGenerator(generatorConfig())
			got, err := generate(expectedAST, GenerateOptions{})
			if err != nil {
				t.Fatalf("generate(expectedAST, GenerateOptions{}): %v", err)
			}
			want, err := Generate(expectedAST, GenerateOptions{})
			if err != nil {
				t.Fatalf("Generate(expectedAST, GenerateOptions{}): %v", err)
			}
			if got != want {
				t.Errorf("generator Generate(stringify.ast) = %q, want %q", got, want)
			}
		})

		t.Run("walker", func(t *testing.T) {
			walker, err := CreateWalker(walkerConfig())
			if err != nil {
				t.Fatalf("CreateWalker(walkerConfig()): %v", err)
			}
			var actualTypes []string
			var expectedTypes []string

			if err := walker.Walk(expectedAST, WalkOptions{
				Enter: func(_ *WalkContext, node Node, _ *ListItem[Node], _ *List[Node]) WalkAction {
					actualTypes = append(actualTypes, node.Type())
					return WalkContinue
				},
			}); err != nil {
				t.Fatalf("walker.Walk(expectedAST, ...): %v", err)
			}
			if err := Walk(expectedAST, WalkOptions{
				Enter: func(_ *WalkContext, node Node, _ *ListItem[Node], _ *List[Node]) WalkAction {
					expectedTypes = append(expectedTypes, node.Type())
					return WalkContinue
				},
			}); err != nil {
				t.Fatalf("Walk(expectedAST, ...): %v", err)
			}

			if !slices.Equal(actualTypes, expectedTypes) {
				t.Errorf("walker types = %q, want %q", actualTypes, expectedTypes)
			}
		})

		t.Run("convertor", func(t *testing.T) {
			// upstream converts in place and checks instanceof List and
			// Array.isArray; the Go plain object is a separate value whose
			// lists are []any, and FromPlainObject builds *List[Node]
			ast, err := Parse(css, ParseOptions{})
			if err != nil {
				t.Fatalf("Parse(%q, ParseOptions{}): %v", css, err)
			}
			findFirstAtrule := func(ast Node) *Atrule {
				atrule, _ := Find(ast, func(_ *WalkContext, node Node, _ *ListItem[Node], _ *List[Node]) bool {
					return node.Type() == "Atrule"
				}).(*Atrule)
				return atrule
			}

			if ast.(*StyleSheet).Children == nil || findFirstAtrule(ast).Prelude.(*AtrulePrelude).Children == nil {
				t.Fatalf("Parse() children are not lists")
			}

			plain, err := ToPlainObject(ast)
			if err != nil {
				t.Fatalf("ToPlainObject(ast): %v", err)
			}
			if _, ok := plain["children"].([]any); !ok {
				t.Errorf("ToPlainObject().children = %T, want []any", plain["children"])
			}

			back, err := FromPlainObject(plain)
			if err != nil {
				t.Fatalf("FromPlainObject(plain): %v", err)
			}
			if back.(*StyleSheet).Children == nil || findFirstAtrule(back).Prelude.(*AtrulePrelude).Children == nil {
				t.Errorf("FromPlainObject() children are not lists")
			}

			_ = []any{(*Convertor).FromPlainObject, (*Convertor).ToPlainObject}
		})

		t.Run("lexer", func(t *testing.T) {
			// upstream: css-tree/lexer exports only the Lexer class
			_ = []any{NewLexer, (*Lexer).Match}
		})

		t.Run("definitionSyntax", func(t *testing.T) {
			_ = []any{
				(*definitionsyntax.SyntaxError).Error,
				definitionsyntax.Generate,
				definitionsyntax.Parse,
				definitionsyntax.Walk,
			}
		})

		t.Run("data", func(t *testing.T) {
			data, err := defaultDefinitions()
			if err != nil {
				t.Fatalf("defaultDefinitions(): %v", err)
			}
			if len(data.Atrules) == 0 || len(data.Properties) == 0 || len(data.Types) == 0 {
				t.Errorf("definitions have %d atrules, %d properties, %d types, want some of each", len(data.Atrules), len(data.Properties), len(data.Types))
			}
		})

		t.Run("data-patch", func(t *testing.T) {
			var dataPatch map[string]json.RawMessage
			if err := json.Unmarshal(patchJSON, &dataPatch); err != nil {
				t.Fatalf("json.Unmarshal(patchJSON, &dataPatch): %v", err)
			}
			got := slices.Sorted(maps.Keys(dataPatch))
			want := []string{"atrules", "properties", "types"}
			if !slices.Equal(got, want) {
				t.Errorf("patch.json keys = %q, want %q", got, want)
			}
		})

		t.Run("utils", func(t *testing.T) {
			_ = []any{
				new(List[Node]),
				Clone,
				DecodeIdent, EncodeIdent,
				IsCustomProperty,
				Keyword,
				Property,
				DecodeString, EncodeString,
				DecodeURL, EncodeURL,
				VendorPrefix,
			}
			if got := EncodeString("foo", false); got != `"foo"` {
				t.Errorf(`EncodeString("foo") = %q, want %q`, got, `"foo"`)
			}
			if got := Keyword("-webkit-foo").Basename; got != "foo" {
				t.Errorf(`Keyword("-webkit-foo").Basename = %q, want %q`, got, "foo")
			}
		})
	})
}
