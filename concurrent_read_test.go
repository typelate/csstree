// Go only.

package csstree

import (
	"os"
	"sync"
	"testing"
)

// TestConcurrentReads checks that the read-only operations may run on one
// AST from several goroutines (run with -race): they iterate its lists
// without the cursors that Walk records in them.
func TestConcurrentReads(t *testing.T) {
	source, err := os.ReadFile("testdata/stringify.css")
	if err != nil {
		t.Fatal(err)
	}
	ast, err := Parse(string(source), ParseOptions{Positions: true})
	if err != nil {
		t.Fatal(err)
	}
	want, err := Generate(ast, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	isDeclaration := func(_ *WalkContext, node Node, _ *ListItem[Node], _ *List[Node]) bool {
		_, ok := node.(*Declaration)
		return ok
	}
	lexer := DefaultLexer()

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if got, err := Generate(ast, GenerateOptions{}); err != nil || got != want {
				t.Errorf("Generate(ast) = %q, %v, want %q", got, err, want)
			}
			if _, err := Generate(ast, GenerateOptions{SourceMap: new(SourceMapGenerator)}); err != nil {
				t.Errorf("Generate(ast) with a source map: %v", err)
			}
			if Find(ast, isDeclaration) == nil || FindLast(ast, isDeclaration) == nil || len(FindAll(ast, isDeclaration)) == 0 {
				t.Error("Find, FindLast or FindAll found no declaration")
			}
			if got, err := Generate(Clone(ast), GenerateOptions{}); err != nil || got != want {
				t.Errorf("Generate(Clone(ast)) = %q, %v, want %q", got, err, want)
			}
			if _, err := ToPlainObject(ast); err != nil {
				t.Errorf("ToPlainObject(ast): %v", err)
			}
			if _, err := lexer.CheckStructure(ast); err != nil {
				t.Errorf("CheckStructure(ast): %v", err)
			}
			if _, err := lexer.FindAllFragments(ast, "type", "color"); err != nil {
				t.Errorf("FindAllFragments(ast): %v", err)
			}
			for _, declaration := range FindAll(ast, isDeclaration) {
				lexer.MatchDeclaration(declaration)
			}
		})
	}
	wg.Wait()
}
