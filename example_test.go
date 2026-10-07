package csstree_test

import (
	"fmt"
	"log"

	"github.com/typelate/csstree"
)

func Example() {
	ast, err := csstree.Parse(`.a { color: red; margin: 0 auto }`, csstree.ParseOptions{})
	if err != nil {
		log.Fatal(err)
	}

	err = csstree.Walk(ast, csstree.WalkOptions{
		Visit: "Declaration",
		Enter: func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem[csstree.Node], _ *csstree.List[csstree.Node]) csstree.WalkAction {
			fmt.Println("property:", node.(*csstree.Declaration).Property)
			return csstree.WalkContinue
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	css, err := csstree.Generate(ast, csstree.GenerateOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(css)

	lexer := csstree.DefaultLexer()
	if _, err := lexer.MatchPropertyString("color", "red"); err == nil {
		fmt.Println("red is a color")
	}
	if _, err := lexer.MatchPropertyString("color", "10px"); err != nil {
		fmt.Println(err)
	}

	// Output:
	// property: color
	// property: margin
	// .a{color:red;margin:0 auto}
	// red is a color
	// Mismatch
	//   syntax: <color>
	//    value: 10px
	//   --------^
}
