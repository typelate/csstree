// Ported from lib/__tests/fixture/definition-syntax.js; the loader is shared
// with the root package in internal/fixturetest.
//
// forEachTest, forEachAtrulePreludeTest and forEachAtruleDescriptorTest need a
// lexer; they belong with the lexer tests in the root package.

package definitionsyntax_test

import (
	"testing"

	"github.com/typelate/csstree/internal/fixturetest"
)

// loadDefinitionSyntaxTests mirrors the `tests` export upstream: every fixture
// file except atrules.json, in directory order, with tests in key order.
func loadDefinitionSyntaxTests(t *testing.T) []fixturetest.DefinitionSyntaxFile {
	t.Helper()
	return fixturetest.LoadDefinitionSyntaxTests(t, "..")
}
