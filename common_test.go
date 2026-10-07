// Ported from lib/__tests/common.js.

package csstree

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/typelate/csstree/tokenizer"
)

const stringifyFixtureFilename = "testdata/stringify.css"

// newlineNormalizer is upstream's str.replace(/\n|\r\n?|\f/g, '\n'): a
// Replacer tries "\r\n" before "\r".
var newlineNormalizer = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\f", "\n")

func normalizeNewlines(str string) string {
	return newlineNormalizer.Replace(str)
}

func TestCommon(t *testing.T) {
	data, err := os.ReadFile(stringifyFixtureFilename)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", stringifyFixtureFilename, err)
	}
	fixture := normalizeNewlines(string(data))

	t.Run("should expose version", func(t *testing.T) {
		// upstream compares with the version of package.json
		if want := "3.2.1"; Version != want {
			t.Errorf("Version = %q, want %q", Version, want)
		}
	})

	t.Run("JSON.stringify()", func(t *testing.T) {
		ast, err := Parse(fixture, ParseOptions{
			Filename:  filepath.Base(stringifyFixtureFilename),
			Positions: true,
		})
		if err != nil {
			t.Fatalf("Parse(%q, ...): %v", fixture, err)
		}

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "    ")
		if err := enc.Encode(ast); err != nil {
			t.Fatalf("enc.Encode(ast): %v", err)
		}

		expectedData, err := os.ReadFile(strings.TrimSuffix(stringifyFixtureFilename, ".css") + ".ast")
		if err != nil {
			t.Fatalf("os.ReadFile(...): %v", err)
		}
		expected := normalizeNewlines(strings.TrimSpace(string(expectedData)))
		actual := strings.TrimSuffix(buf.String(), "\n")

		if actual != expected {
			actualLines := strings.Split(actual, "\n")
			expectedLines := strings.Split(expected, "\n")
			for i := 0; i < len(actualLines) && i < len(expectedLines); i++ {
				if actualLines[i] != expectedLines[i] {
					t.Fatalf("JSON(Parse(stringify.css)) differs at line %d:\n got %q\nwant %q", i+1, actualLines[i], expectedLines[i])
				}
			}
			t.Fatalf("JSON(Parse(stringify.css)) has %d lines, want %d", len(actualLines), len(expectedLines))
		}
	})

	t.Run("test CSS should contain all node types", func(t *testing.T) {
		foundTypes := make(map[string]bool)
		ast, err := Parse(fixture, ParseOptions{})
		if err != nil {
			t.Fatalf("Parse(%q, ParseOptions{}): %v", fixture, err)
		}

		if err := Walk(ast, WalkOptions{
			Enter: func(_ *WalkContext, node Node, _ *ListItem[Node], _ *List[Node]) WalkAction {
				foundTypes[node.Type()] = true
				return WalkContinue
			},
		}); err != nil {
			t.Fatalf("Walk(ast, ...): %v", err)
		}

		var actual []string
		for nodeType := range foundTypes {
			actual = append(actual, nodeType)
		}
		slices.Sort(actual)

		var types []string
		for nodeType := range nodeConfigs() {
			// DeclarationList doesn't appear in StyleSheet
			// FIXME: temporary filter white space
			if nodeType != "DeclarationList" && nodeType != "WhiteSpace" {
				types = append(types, nodeType)
			}
		}
		slices.Sort(types)

		if !slices.Equal(actual, types) {
			t.Errorf("Walk(Parse(stringify.css)) types = %v, want %v", actual, types)
		}
	})

	t.Run("extension in base classes should not cause to exception", func(t *testing.T) {
		// Upstream extends Object.prototype and Array.prototype, which
		// has no Go counterpart; the fork itself is ported.
		t.Run("fork()", func(t *testing.T) {
			if _, err := Fork(&Config{
				Node: map[string]NodeConfig{
					"Test": {
						Structure: []StructureField{
							{Name: "foo", Types: []FieldType{nodeField("Rule")}},
							{Name: "bar", Types: []FieldType{listField("Rule")}},
						},
					},
				},
			}); err != nil {
				t.Errorf("Fork() error = %v", err)
			}
		})
	})

	t.Run("generic option should work in fork()", func(t *testing.T) {
		forkWithoutGeneric, err := Fork(&Config{
			// By default, generic is true, but we disable it
			Generic: new(false),
		})
		if err != nil {
			t.Fatalf("Fork(...): %v", err)
		}

		forkWithGeneric, err := Fork(&Config{
			// By default, generic is true
		})
		if err != nil {
			t.Fatalf("Fork(...): %v", err)
		}

		// Generic should be set
		if forkWithoutGeneric.Lexer().Generic() != false {
			t.Errorf("forkWithoutGeneric.Lexer.Generic() = true, want false")
		}
		if forkWithGeneric.Lexer().Generic() != true {
			t.Errorf("forkWithGeneric.Lexer.Generic() = false, want true")
		}

		// Lexer match should depend on generic (<length> is generic type)
		node, err := Parse("1px", ParseOptions{Context: "value"})
		if err != nil {
			t.Fatalf(`Parse(%q, ParseOptions{Context: "value"}): %v`, "1px", err)
		}

		// upstream asserts that match throws; the Go lexer reports what
		// upstream throws as the result's Error, not a *SyntaxMatchError
		var matchError *SyntaxMatchError
		if _, err := forkWithoutGeneric.Lexer().Match("<length>", node); err == nil || errors.As(err, &matchError) {
			t.Errorf(`forkWithoutGeneric.Lexer.Match("<length>", 1px) error = %v, want "Bad syntax reference: <length>"`, err)
		}
		if _, err := forkWithGeneric.Lexer().Match("<length>", node); err != nil {
			t.Errorf(`forkWithGeneric.Lexer.Match("<length>", 1px) error = %v, want nil`, err)
		}
	})

	t.Run("fork()", func(t *testing.T) {
		t.Run("extend nodes", func(t *testing.T) {
			ast, err := Parse("20px", ParseOptions{Context: "value"})
			if err != nil {
				t.Fatalf(`Parse(%q, ParseOptions{Context: "value"}): %v`, "20px", err)
			}
			forkedSyntax, err := Fork(&Config{
				Node: map[string]NodeConfig{
					"Dimension": {
						Generate: func(g *Generator, node Node) {
							g.Token(tokenizer.Dimension, "?"+node.(*Dimension).Unit)
						},
					},
				},
			})
			if err != nil {
				t.Fatalf("Fork(...): %v", err)
			}

			if actual, err := forkedSyntax.Generate(ast, GenerateOptions{}); err != nil || actual != "?px" {
				t.Errorf("forkedSyntax.Generate(20px) = %q, %v, want %q", actual, err, "?px")
			}
			if actual, err := Generate(ast, GenerateOptions{}); err != nil || actual != "20px" {
				t.Errorf("Generate(20px) = %q, %v, want %q", actual, err, "20px")
			}
		})
	})
}
