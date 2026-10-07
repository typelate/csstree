package definitionsyntax_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/typelate/csstree/definitionsyntax"
)

func ExampleParse() {
	group, err := definitionsyntax.Parse(`<length [0,∞]>{1,4} | auto`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%q\n", group.Combinator)
	for _, term := range group.Terms {
		switch term := term.(type) {
		case *definitionsyntax.Multiplier:
			typ := term.Term.(*definitionsyntax.Type)
			fmt.Println(term.Type(), term.Min, term.Max, typ.Name, typ.Opts.Min.Value, typ.Opts.Max == nil)
		case *definitionsyntax.Keyword:
			fmt.Println(term.Type(), term.Name)
		}
	}

	_, err = definitionsyntax.Parse(`<length`)
	var syntaxErr *definitionsyntax.SyntaxError
	if errors.As(err, &syntaxErr) {
		fmt.Println(syntaxErr.RawMessage, syntaxErr.Offset)
	}

	// Output:
	// "|"
	// Multiplier 1 4 length 0 true
	// Keyword auto
	// Expect `>` 7
}

func ExampleGenerate() {
	group, err := definitionsyntax.Parse(`[a|b]  ,  <length>#`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(definitionsyntax.Generate(group, definitionsyntax.GenerateOptions{}))
	fmt.Println(definitionsyntax.Generate(group, definitionsyntax.GenerateOptions{Compact: true}))
	fmt.Println(definitionsyntax.Generate(group, definitionsyntax.GenerateOptions{
		Decorate: func(str string, node definitionsyntax.Node) string {
			if _, ok := node.(*definitionsyntax.Type); ok {
				return "{" + str + "}"
			}
			return str
		},
	}))

	// Output:
	// [ a | b ] , <length>#
	// [a|b] , <length>#
	// [ a | b ] , {<length>}#
}
