package csstree

import (
	"sync"
	"testing"
)

// TestLexerConcurrentMatch checks that the shared default lexer can match
// from several goroutines while its syntaxes are parsed on first use (Go
// only; run with -race).
func TestLexerConcurrentMatch(t *testing.T) {
	lexer, err := CreateLexer(&Config{
		Generic: new(true),
		Types: map[string]string{
			"foo": "<bar>#",
			"bar": "[ 1 | 2 | 3 ] | <length>",
		},
		Properties: map[string]string{
			"test":  "<foo> && <'other'>?",
			"other": "a | b",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for _, value := range []string{"1, 2px", "1 a", "b 3", "x"} {
				lexer.MatchPropertyString("test", value)
				DefaultLexer().MatchPropertyString("background", value)
			}
		})
	}
	wg.Wait()
}
