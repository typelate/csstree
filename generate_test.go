// Ported from lib/__tests/generate.js.

package csstree

import (
	"fmt"
	"strings"
	"testing"
)

// unknownTestNode is upstream's { type: 'xxx' }.
type unknownTestNode struct{}

func (unknownTestNode) Type() string        { return "xxx" }
func (unknownTestNode) Location() *Location { return nil }

// createGenerateTests runs upstream's three tests of a fixture, "generate",
// "(plain object)" and "(round-trip)", on one parse of the source (Go's
// ToPlainObject and Generate do not change the AST they are given).
func createGenerateTests(t *testing.T, test astFixtureTest) {
	if test.Skip {
		t.Skip("skipped by fixture")
	}

	expected := test.Source
	if test.Generate != nil {
		expected = *test.Generate
	}

	ast, err := Parse(test.Source, test.ParseOptions(false))
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", test.Source, err)
	}

	// generate: strings should be equal
	generated, err := Generate(ast, GenerateOptions{})
	if err != nil {
		t.Fatalf("Generate(Parse(%q)) error = %v", test.Source, err)
	}
	if generated != expected {
		t.Errorf("Generate(Parse(%q)) = %q, want %q", test.Source, generated, expected)
	}

	// (plain object): strings should be equal
	if actual, err := generatePlainObject(ast); err != nil {
		t.Errorf("Generate(FromPlainObject(ToPlainObject(Parse(%q)))) error = %v", test.Source, err)
	} else if actual != expected {
		t.Errorf("Generate(FromPlainObject(ToPlainObject(Parse(%q)))) = %q, want %q", test.Source, actual, expected)
	}

	// (round-trip)
	if test.SkipRoundtrip {
		t.Log("round-trip skipped by fixture")
		return
	}
	actualAST, err := Parse(generated, test.ParseOptions(false))
	if err != nil {
		t.Fatalf("Parse(Generate(Parse(%q))) = Parse(%q) error = %v", test.Source, generated, err)
	}

	// https://drafts.csswg.org/css-syntax/#serialization
	// The only requirement for serialization is that it must "round-trip" with parsing,
	// that is, parsing the stylesheet must produce the same data structures as parsing,
	// serializing, and parsing again, except for consecutive <whitespace-token>s,
	// which may be collapsed into a single token.
	assertJSONEqual(t, fmt.Sprintf("Parse(Generate(Parse(%q)))", test.Source), toPlainJSON(t, actualAST), toPlainJSON(t, ast))
}

// generatePlainObject generates CSS from the plain object of ast. Go's
// Generate takes a typed AST, so the plain object is converted back with
// FromPlainObject.
func generatePlainObject(ast Node) (string, error) {
	plainObject, err := ToPlainObject(ast)
	if err != nil {
		return "", fmt.Errorf("ToPlainObject: %w", err)
	}
	converted, err := FromPlainObject(plainObject)
	if err != nil {
		return "", fmt.Errorf("FromPlainObject: %w", err)
	}
	return Generate(converted, GenerateOptions{})
}

func createGenerateWithSourceMapTest(t *testing.T, test astFixtureTest) {
	if test.Skip {
		t.Skip("skipped by fixture")
	}

	options := test.ParseOptions(false)
	options.Positions = true
	ast, err := Parse(test.Source, options)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", test.Source, err)
	}

	actual, err := Generate(ast, GenerateOptions{SourceMap: new(SourceMapGenerator)})
	if err != nil {
		t.Fatalf("Generate(Parse(%q), {sourceMap: true}) error = %v", test.Source, err)
	}

	// strings should be equal
	expected := test.Source
	if test.Generate != nil {
		expected = *test.Generate
	}
	if actual != expected {
		t.Errorf("Generate(Parse(%q), {sourceMap: true}) = %q, want %q", test.Source, actual, expected)
	}
}

func TestGenerate(t *testing.T) {
	forEachASTTest(t, false, createGenerateTests)

	t.Run("should throws on unknown node type", func(t *testing.T) {
		_, err := Generate(unknownTestNode{}, GenerateOptions{})
		if err == nil || !strings.Contains(err.Error(), "Unknown node type") {
			t.Errorf("Generate({type: 'xxx'}) error = %v, want Unknown node type", err)
		}
	})

	t.Run("sourceMap", func(t *testing.T) {
		forEachASTTest(t, false, createGenerateWithSourceMapTest)

		t.Run("should generate a map", func(t *testing.T) {
			ast, err := Parse(".a {\n  color: red;\n}\n", ParseOptions{
				Filename:  "test.css",
				Positions: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			sourceMap := new(SourceMapGenerator)
			css, err := Generate(ast, GenerateOptions{SourceMap: sourceMap})
			if err != nil {
				t.Fatal(err)
			}

			if want := ".a{color:red}"; css != want {
				t.Errorf("Generate() css = %q, want %q", css, want)
			}
			if want := `{"version":3,"sources":["test.css"],"names":[],"mappings":"AAAA,E,CACE,S"}`; sourceMap.String() != want {
				t.Errorf("Generate() map = %s, want %s", sourceMap, want)
			}
		})

		t.Run("complex CSS", func(t *testing.T) {
			ast, err := Parse(".a { color: #ff0000; } .b { display: block; float: left; } @media foo { .c { color: red } }", ParseOptions{
				Filename:  "test.css",
				Positions: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			sourceMap := new(SourceMapGenerator)
			css, err := Generate(ast, GenerateOptions{SourceMap: sourceMap})
			if err != nil {
				t.Fatal(err)
			}

			if want := ".a{color:#ff0000}.b{display:block;float:left}@media foo{.c{color:red}}"; css != want {
				t.Errorf("Generate() css = %q, want %q", css, want)
			}
			if want := `{"version":3,"sources":["test.css"],"names":[],"mappings":"AAAA,E,CAAK,a,CAAkB,E,CAAK,a,CAAgB,U,CAAe,WAAa,E,CAAK,W"}`; sourceMap.String() != want {
				t.Errorf("Generate() map = %s, want %s", sourceMap, want)
			}
		})
	})

	t.Run("spec mode", func(t *testing.T) {
		// One check per fixture entry, without a subtest each: there are
		// thousands of them and each message names its fixture.ID.
		for _, fixture := range generateAutoEmitWSFixture() {
			leftValue := fixture.Left.Value
			rightValue := fixture.Right.Value
			expectedNoWs := leftValue + rightValue
			expectedWs := leftValue + " " + rightValue

			children := new(List[Node])
			children.AppendData(&Raw{Value: leftValue})
			children.AppendData(&Raw{Value: rightValue})
			actual, err := Generate(&Value{Children: children}, GenerateOptions{Mode: "spec"})
			if err != nil {
				t.Errorf("%s: Generate(%q %q) error = %v", fixture.ID, leftValue, rightValue, err)
				continue
			}

			if fixture.WS {
				if actual != expectedWs {
					t.Errorf("%s: Generate(%q %q) = %q, want %q", fixture.ID, leftValue, rightValue, actual, expectedWs)
				}
			} else {
				if actual != expectedNoWs && actual != expectedWs {
					t.Errorf("%s: Generate(%q %q) = %q, want %q or %q", fixture.ID, leftValue, rightValue, actual, expectedNoWs, expectedWs)
				}
			}
		}
	})

	t.Run("auto-emit whitespace", func(t *testing.T) {
		t.Run("should auto insert whitespaces where necessary", func(t *testing.T) {
			const source = "span#foo { border: 1%var(--a)#ff0000; }"
			ast, err := Parse(source, ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}

			actual, err := Generate(ast, GenerateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if want := "span#foo{border:1% var(--a) #ff0000}"; actual != want {
				t.Errorf("Generate(Parse(%q)) = %q, want %q", source, actual, want)
			}
		})

		t.Run("should not insert whitespaces for values passed to tokenize() by default", func(t *testing.T) {
			const source = "span#foo { border: 1%var(--a)#ff0000; }"
			ast, err := Parse(source, ParseOptions{
				NoParseRulePrelude: true,
				NoParseValue:       true,
			})
			if err != nil {
				t.Fatal(err)
			}

			actual, err := Generate(ast, GenerateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if want := "span#foo{border:1%var(--a)#ff0000}"; actual != want {
				t.Errorf("Generate(Parse(%q)) = %q, want %q", source, actual, want)
			}
		})
	})
}
