// Ported from lib/__tests/fixture/definition-syntax.js. The fixture loader
// and JsonLocator are shared with the definitionsyntax package in
// internal/fixturetest; forEachTest, forEachAtrulePreludeTest and
// forEachAtruleDescriptorTest need a lexer, so they live here.

package csstree

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/typelate/csstree/internal/fixturetest"
)

const definitionSyntaxFixtureDir = fixturetest.DefinitionSyntaxDir

// definitionSyntaxLexerConfig is the `lexer` of a fixture test, passed to
// createLexer.
type definitionSyntaxLexerConfig struct {
	Generic         *bool               `json:"generic"`
	CSSWideKeywords []string            `json:"cssWideKeywords"`
	Units           map[string][]string `json:"units"`
	Types           map[string]string   `json:"types"`
	Properties      map[string]string   `json:"properties"`
}

// fixtureLexer returns the lexer a definition syntax fixture test runs with:
// the default lexer, or one created from the test's `lexer` config.
func fixtureLexer(t *testing.T, test fixturetest.DefinitionSyntaxTest) *Lexer {
	t.Helper()
	var config *definitionSyntaxLexerConfig
	if len(test.Lexer) > 0 {
		if err := json.Unmarshal(test.Lexer, &config); err != nil {
			t.Fatalf("%s: decoding lexer config: %v", test.Name, err)
		}
	}
	if config == nil {
		return DefaultLexer()
	}
	lexer, err := CreateLexer(&Config{
		Generic:         config.Generic,
		CSSWideKeywords: config.CSSWideKeywords,
		Units:           config.Units,
		Types:           config.Types,
		Properties:      config.Properties,
	})
	if err != nil {
		t.Fatalf("%s: CreateLexer() error = %v", test.Name, err)
	}
	return lexer
}

// definitionSyntaxFactory receives a fixture value: testType is "valid" or
// "invalid", testState the fixture's `test` ("skip", "only" or ""), syntax
// the definition syntax to match ("" to match property instead).
type definitionSyntaxFactory func(t *testing.T, testType, testState, name string, lexer *Lexer, property, value, syntax string)

// runDefinitionSyntaxValue runs one fixture value as a subtest named name,
// skipping it when the fixture says so (upstream it.skip).
func runDefinitionSyntaxValue(t *testing.T, testState, name string, fn func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		if testState == "skip" {
			t.Skip("skipped by fixture")
		}
		fn(t)
	})
}

// forEachDefinitionSyntaxTest mirrors forEachTest.
func forEachDefinitionSyntaxTest(t *testing.T, factory definitionSyntaxFactory) {
	t.Helper()

	for _, file := range fixturetest.LoadDefinitionSyntaxTests(t, ".") {
		for _, test := range file.Tests {
			syntax := test.Syntax
			lexer := fixtureLexer(t, test)

			for _, field := range []struct {
				name   string
				values []string
			}{
				{"valid", test.Valid},
				{"invalid", test.Invalid},
			} {
				for idx, value := range field.values {
					factory(
						t,
						field.name,
						test.Test,
						fmt.Sprintf("%s %s#%d", test.Name, field.name, idx),
						lexer,
						test.Property,
						value,
						syntax,
					)
				}
			}
		}
	}
}

// atruleTest is an entry of atrules.json.
type atruleTest struct {
	Name        string
	Test        string
	Prelude     *atruleTestValues
	Descriptors map[string]atruleTestValues
}

type atruleTestValues struct {
	Valid   []string `json:"valid"`
	Invalid []string `json:"invalid"`
}

// loadAtruleTests returns atrules.json's tests in key order (upstream
// atruleTests).
func loadAtruleTests(t *testing.T) ([]string, map[string]atruleTest) {
	t.Helper()

	filename := filepath.Join(definitionSyntaxFixtureDir, "atrules.json")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", filename, err)
	}
	locator, err := fixturetest.NewJSONLocator(path.Join(definitionSyntaxFixtureDir, "atrules.json"), data)
	if err != nil {
		t.Fatalf("fixturetest.NewJSONLocator(..., data): %v", err)
	}

	var raw map[string]struct {
		Test        string                      `json:"test"`
		Prelude     *atruleTestValues           `json:"prelude"`
		Descriptors map[string]atruleTestValues `json:"descriptors"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("%s: %v", filename, err)
	}

	tests := make(map[string]atruleTest)
	for _, key := range locator.Keys() {
		name, err := locator.Get(key)
		if err != nil {
			t.Fatalf("locator.Get(%q): %v", key, err)
		}
		tests[key] = atruleTest{
			Name:        name,
			Test:        raw[key].Test,
			Prelude:     raw[key].Prelude,
			Descriptors: raw[key].Descriptors,
		}
	}

	return locator.Keys(), tests
}

// forEachAtrulePreludeTest mirrors forEachAtrulePreludeTest.
func forEachAtrulePreludeTest(t *testing.T, factory func(t *testing.T, testType, testState, name string, lexer *Lexer, atruleName, value string)) {
	t.Helper()

	atruleNames, atruleTests := loadAtruleTests(t)
	for _, atruleName := range atruleNames {
		testset := atruleTests[atruleName]
		if testset.Prelude == nil {
			continue
		}

		test := testset.Prelude
		for _, field := range []struct {
			name   string
			values []string
		}{
			{"valid", test.Valid},
			{"invalid", test.Invalid},
		} {
			for idx, value := range field.values {
				factory(
					t,
					field.name,
					testset.Test,
					fmt.Sprintf("%s @%s prelude %s#%d", testset.Name, atruleName, field.name, idx),
					DefaultLexer(),
					atruleName,
					value,
				)
			}
		}
	}
}

// forEachAtruleDescriptorTest mirrors forEachAtruleDescriptorTest;
// descriptors are visited in name order.
func forEachAtruleDescriptorTest(t *testing.T, factory func(t *testing.T, testType, testState, name string, lexer *Lexer, atruleName, descriptorName, value string)) {
	t.Helper()

	atruleNames, atruleTests := loadAtruleTests(t)
	for _, atruleName := range atruleNames {
		testset := atruleTests[atruleName]
		if testset.Descriptors == nil {
			continue
		}

		for _, descriptorName := range sortedKeys(testset.Descriptors) {
			test := testset.Descriptors[descriptorName]
			for _, field := range []struct {
				name   string
				values []string
			}{
				{"valid", test.Valid},
				{"invalid", test.Invalid},
			} {
				for idx, value := range field.values {
					factory(
						t,
						field.name,
						testset.Test,
						fmt.Sprintf("%s @%s descriptor %s %s#%d", testset.Name, atruleName, descriptorName, field.name, idx),
						DefaultLexer(),
						atruleName,
						descriptorName,
						value,
					)
				}
			}
		}
	}
}
