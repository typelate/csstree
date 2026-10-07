// Package fixturetest loads the JSON test fixtures under testdata/ that the
// tests of more than one package of the module read.
//
// It is test support: import it only from _test.go files.
package fixturetest

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"testing"
)

// DefinitionSyntaxDir is the directory of the definition syntax fixtures,
// relative to the module root.
const DefinitionSyntaxDir = "testdata/definition-syntax"

// DefinitionSyntaxTest is one entry of a definition syntax fixture file.
type DefinitionSyntaxTest struct {
	Key string
	// Name is upstream's test name, "file:line:column (key)".
	Name string
	// Test is the fixture's `test`: "skip", "only" or "".
	Test     string
	Syntax   string
	Property string
	Type     string
	// Lexer is the fixture's `lexer` config, nil if it has none.
	Lexer   json.RawMessage
	Valid   []string
	Invalid []string
}

// DefinitionSyntaxFile is a definition syntax fixture file.
type DefinitionSyntaxFile struct {
	Filename string
	Tests    []DefinitionSyntaxTest
}

// LoadDefinitionSyntaxTests mirrors the `tests` export upstream: every
// fixture file except atrules.json, in directory order, with tests in key
// order. moduleRoot is the path of the module root from the test's working
// directory.
func LoadDefinitionSyntaxTests(t testing.TB, moduleRoot string) []DefinitionSyntaxFile {
	t.Helper()

	dir := filepath.Join(moduleRoot, filepath.FromSlash(DefinitionSyntaxDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading definition syntax fixtures: %v", err)
	}

	var result []DefinitionSyntaxFile
	for _, entry := range entries {
		fn := entry.Name()
		if filepath.Ext(fn) != ".json" || fn == "atrules.json" {
			continue
		}

		filename := filepath.Join(dir, fn)
		data, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("reading definition syntax fixture: %v", err)
		}

		// named as upstream names it, relative to the module root
		locator, err := NewJSONLocator(path.Join(DefinitionSyntaxDir, fn), data)
		if err != nil {
			t.Fatalf("NewJSONLocator(%q): %v", filename, err)
		}

		var raw map[string]struct {
			Test     string          `json:"test"`
			Syntax   *string         `json:"syntax"`
			Property *string         `json:"property"`
			Type     *string         `json:"type"`
			Lexer    json.RawMessage `json:"lexer"`
			Valid    []string        `json:"valid"`
			Invalid  []string        `json:"invalid"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("%s: %v", filename, err)
		}

		file := DefinitionSyntaxFile{Filename: filename}
		for _, key := range locator.Keys() {
			test := raw[key]
			name, err := locator.Get(key)
			if err != nil {
				t.Fatalf("locator.Get(%q): %v", key, err)
			}

			item := DefinitionSyntaxTest{
				Key:     key,
				Name:    name,
				Test:    test.Test,
				Lexer:   test.Lexer,
				Valid:   test.Valid,
				Invalid: test.Invalid,
			}
			if test.Syntax != nil {
				item.Syntax = *test.Syntax
			}
			if test.Property != nil {
				item.Property = *test.Property
			}
			if test.Type != nil {
				item.Type = *test.Type
			}

			if item.Syntax == "" && item.Property == "" && item.Type == "" {
				item.Syntax = key
			}

			file.Tests = append(file.Tests, item)
		}

		result = append(result, file)
	}

	return result
}
