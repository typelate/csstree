// Ported from the parts of lib/__tests/common.js that need only the parser.

package csstree

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const stringifyFixtureFilename = "testdata/stringify.css"

func normalizeNewlines(str string) string {
	return regexp.MustCompile(`\n|\r\n?|\f`).ReplaceAllString(str, "\n")
}

func TestCommon(t *testing.T) {
	data, err := os.ReadFile(stringifyFixtureFilename)
	if err != nil {
		t.Fatal(err)
	}
	fixture := normalizeNewlines(string(data))

	t.Run("should expose version", func(t *testing.T) {
		t.Skip("not ported yet: version")
	})

	t.Run("JSON.stringify()", func(t *testing.T) {
		ast, err := Parse(fixture, ParseOptions{
			Filename:  filepath.Base(stringifyFixtureFilename),
			Positions: true,
		})
		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "    ")
		if err := enc.Encode(ast); err != nil {
			t.Fatal(err)
		}

		expectedData, err := os.ReadFile(strings.TrimSuffix(stringifyFixtureFilename, ".css") + ".ast")
		if err != nil {
			t.Fatal(err)
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
		t.Skip("not ported yet: needs walker")
	})

	t.Run("extension in base classes should not cause to exception", func(t *testing.T) {
		t.Skip("not ported: JavaScript prototype extension has no Go counterpart")
	})

	t.Run("generic option should work in fork()", func(t *testing.T) {
		t.Skip("not ported yet: needs fork and lexer")
	})

	t.Run("fork()", func(t *testing.T) {
		t.Run("extend nodes", func(t *testing.T) {
			t.Skip("not ported yet: needs fork")
		})
	})
}
