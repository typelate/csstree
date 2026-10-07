package csstree

import (
	"os"
	"testing"
)

// BenchmarkMatchProperty matches property values of the default lexer, as
// text, and the declarations of testdata/stringify.css, as parsed ASTs.
func BenchmarkMatchProperty(b *testing.B) {
	lexer := DefaultLexer()

	for _, bench := range []struct {
		name, property, value string
	}{
		{"calc", "width", "calc(100% / 3 - 2 * 1em)"},
		{"background", "background", "url(image.png) no-repeat center / cover, linear-gradient(to right, rgba(255, 255, 255, 0) 0%, #fff 50%) repeat-x 0 0 / 100% 1px border-box, #eee"},
		{"grid", "grid-template-columns", "repeat(auto-fill, minmax(10px, 1fr))"},
		{"rgb", "color", "rgb(0 0 0 / 50%)"},
	} {
		if _, err := lexer.MatchPropertyString(bench.property, bench.value); err != nil {
			b.Fatalf("MatchPropertyString(%q, %q) error = %v", bench.property, bench.value, err)
		}
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				lexer.MatchPropertyString(bench.property, bench.value)
			}
		})
	}

	b.Run("stringify.css", func(b *testing.B) {
		source, err := os.ReadFile("testdata/stringify.css")
		if err != nil {
			b.Fatal(err)
		}
		ast, err := Parse(string(source), ParseOptions{})
		if err != nil {
			b.Fatal(err)
		}
		var declarations []Node
		err = Walk(ast, WalkOptions{
			Visit: "Declaration",
			Enter: func(_ *WalkContext, node Node, _ *ListItem[Node], _ *List[Node]) WalkAction {
				declarations = append(declarations, node)
				return WalkContinue
			},
		})
		if err != nil {
			b.Fatal(err)
		}
		if len(declarations) == 0 {
			b.Fatal("no declarations in testdata/stringify.css")
		}

		b.ReportAllocs()
		for b.Loop() {
			for _, declaration := range declarations {
				lexer.MatchDeclaration(declaration)
			}
		}
	})
}
