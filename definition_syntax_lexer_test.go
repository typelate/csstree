// Ported from the lexer dependent parts of lib/__tests/definition-syntax-parse.js
// ("parse") and lib/__tests/definition-syntax-generate.js ("round trip" /
// "lexer dictionary"). The definitionsyntax package cannot import the lexer,
// so these live in the root package.

package csstree

import (
	"maps"
	"reflect"
	"testing"

	"github.com/typelate/csstree/definitionsyntax"
)

// forEachLexerDictionarySyntax visits every serializable syntax of the
// default lexer as upstream's loops over lexer.properties, lexer.types and
// lexer.atrules do, naming subtests the same way. The subtests of the
// properties, types and atrules run in parallel.
func forEachLexerDictionarySyntax(t *testing.T, fn func(t *testing.T, definition *Descriptor)) {
	lexer := DefaultLexer()

	for _, section := range []string{"properties", "types"} {
		dict := lexer.Properties()
		if section == "types" {
			dict = lexer.Types()
		}
		for name, definition := range dict {
			if definition.Serializable() {
				t.Run(section+"/"+name, func(t *testing.T) {
					t.Parallel()
					fn(t, definition)
				})
			}
		}
	}

	for name, definition := range lexer.Atrules() {
		t.Run("atrules/"+name, func(t *testing.T) {
			t.Parallel()
			if prelude := definition.Prelude(); prelude != nil {
				t.Run("prelude", func(t *testing.T) {
					fn(t, prelude)
				})
			}

			descriptors := maps.Collect(definition.Descriptors())
			if len(descriptors) > 0 {
				t.Run("definitions", func(t *testing.T) {
					for _, name := range sortedKeys(descriptors) {
						t.Run(name, func(t *testing.T) {
							fn(t, descriptors[name])
						})
					}
				})
			}
		})
	}
}

func mustDescriptorSyntax(t *testing.T, definition *Descriptor) definitionsyntax.Node {
	t.Helper()
	syntax, err := definition.Syntax()
	if err != nil {
		t.Fatalf("%s %s: Syntax() error = %v", definition.Kind(), definition.Name(), err)
	}
	return syntax
}

func TestDefinitionSyntaxParseLexer(t *testing.T) {
	// FIXME: in fact this test checks lexer's dictionaries, since lexer uses
	// parse under the hood when first access to definition's "syntax" property;
	// Problems:
	// - test has side effect, because access to syntax trigger parsing and cache the result
	// - some other tests can init syntax
	// - dictionary can move or store actual ast and don't use parsing et all
	// - etc.
	t.Run("parse", func(t *testing.T) {
		forEachLexerDictionarySyntax(t, func(t *testing.T, definition *Descriptor) {
			syntax := mustDescriptorSyntax(t, definition)
			if syntax == nil || syntax.Type() != "Group" {
				t.Errorf("%s %s: Syntax() = %v, want a Group", definition.Kind(), definition.Name(), syntax)
			}
		})
	})
}

func TestDefinitionSyntaxGenerateLexer(t *testing.T) {
	assertGenerateParseRoundTrip := func(t *testing.T, syntax definitionsyntax.Node) {
		t.Helper()
		source := definitionsyntax.Generate(syntax, definitionsyntax.GenerateOptions{})
		actual, err := definitionsyntax.Parse(source)
		if err != nil {
			t.Fatalf("Parse(Generate(syntax)) error = %v (generated %q)", err, source)
		}
		if !reflect.DeepEqual(definitionsyntax.Node(actual), syntax) {
			t.Errorf("Parse(%q) = %v, want %v", source, actual, syntax)
		}
	}

	t.Run("round trip", func(t *testing.T) {
		t.Run("lexer dictionary", func(t *testing.T) {
			forEachLexerDictionarySyntax(t, func(t *testing.T, definition *Descriptor) {
				assertGenerateParseRoundTrip(t, mustDescriptorSyntax(t, definition))
			})
		})
	})
}
