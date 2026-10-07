package csstree

import (
	"testing"
)

var fuzzContexts = []string{"stylesheet", "atrule", "atrulePrelude", "mediaQueryList", "mediaQuery", "condition", "rule", "selectorList", "selector", "block", "declarationList", "declaration", "value"}

func FuzzParse(f *testing.F) {
	for _, file := range loadASTFixtures(f, nil) {
		for _, test := range file.Tests {
			f.Add(test.Source, uint8(0), false)
		}
		for _, test := range file.Errors {
			f.Add(test.Source, uint8(0), false)
		}
	}
	f.Fuzz(func(t *testing.T, source string, context uint8, positions bool) {
		opts := ParseOptions{Context: fuzzContexts[int(context)%len(fuzzContexts)], Positions: positions}
		ast, err := Parse(source, opts)
		if err != nil {
			return
		}
		if _, err := Generate(ast, GenerateOptions{}); err != nil {
			t.Fatalf("Generate after Parse(%q, %+v) = %v", source, opts, err)
		}
	})
}
