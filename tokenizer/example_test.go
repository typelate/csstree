package tokenizer_test

import (
	"fmt"

	"github.com/typelate/csstree/tokenizer"
)

func ExampleTokenize() {
	source := `a{color:#f00}`
	tokenizer.Tokenize(source, func(tokenType tokenizer.TokenType, start, end int) {
		fmt.Printf("%-20s %d-%d %q\n", tokenType, start, end, source[start:end])
	})

	// Output:
	// ident-token          0-1 "a"
	// {-token              1-2 "{"
	// ident-token          2-7 "color"
	// colon-token          7-8 ":"
	// hash-token           8-12 "#f00"
	// }-token              12-13 "}"
}

func ExampleOffsetToLocation() {
	source := "a {\n  color: red;\n}"
	locations := tokenizer.OffsetToLocation{Source: source, Filename: "a.css"}
	start := 6 // the "c" of color
	loc := locations.Range(start, start+len("color"))
	fmt.Println(loc.Filename, loc.Start.Line, loc.Start.Column, loc.End.Column)

	// Output:
	// a.css 2 3 8
}
