package csstree_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/typelate/csstree"
	"github.com/typelate/csstree/tokenizer"
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

func ExampleParse() {
	// ContextValue parses a declaration value, so the root is a *Value.
	// Positions records a Location in every node.
	ast, err := csstree.Parse(`1px solid red`, csstree.ParseOptions{
		Context:   csstree.ContextValue,
		Positions: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	value := ast.(*csstree.Value)
	for node := range value.Children.All() {
		loc := node.Location()
		fmt.Printf("%s at %d:%d\n", node.Type(), loc.Start.Line, loc.Start.Column)
	}

	// A syntax error the parser cannot recover from is a *SyntaxError.
	_, err = csstree.Parse(`a{}`, csstree.ParseOptions{Context: csstree.ContextValue})
	var syntaxErr *csstree.SyntaxError
	if errors.As(err, &syntaxErr) {
		fmt.Println(syntaxErr.Line, syntaxErr.Column, syntaxErr.Message)
	}

	// Output:
	// Dimension at 1:1
	// Identifier at 1:5
	// Identifier at 1:11
	// 1 2 Unexpected input
}

func ExampleGenerate() {
	ast, err := csstree.Parse(`a  >  b { color : RED ; }`, csstree.ParseOptions{})
	if err != nil {
		log.Fatal(err)
	}
	css, err := csstree.Generate(ast, csstree.GenerateOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(css)

	// Output:
	// a>b{color:RED}
}

func ExampleGenerateWithSourceMap() {
	ast, err := csstree.Parse(".a {\n  color: red;\n}\n", csstree.ParseOptions{
		Positions: true,
		Filename:  "a.css",
	})
	if err != nil {
		log.Fatal(err)
	}
	css, sourceMap, err := csstree.GenerateWithSourceMap(ast, csstree.GenerateOptions{})
	if err != nil {
		log.Fatal(err)
	}
	sourceMap.File = "a.min.css"
	fmt.Println(css)

	var decoded struct {
		File     string   `json:"file"`
		Sources  []string `json:"sources"`
		Mappings string   `json:"mappings"`
	}
	if err := json.Unmarshal([]byte(sourceMap.String()), &decoded); err != nil {
		log.Fatal(err)
	}
	fmt.Println(decoded.File, decoded.Sources, decoded.Mappings)

	// Output:
	// .a{color:red}
	// a.min.css [a.css] AAAA,E,CACE,S
}

func ExampleWalk() {
	ast, err := csstree.Parse(`a { color: red } b { color: blue } c { color: green }`, csstree.ParseOptions{})
	if err != nil {
		log.Fatal(err)
	}

	// Visit calls Enter only for Rule nodes. WalkSkip skips the children
	// of the rule for "a", and WalkBreak stops the walk at the rule for
	// "b", so the rule for "c" is not visited.
	err = csstree.Walk(ast, csstree.WalkOptions{
		Visit: "Rule",
		Enter: func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem[csstree.Node], _ *csstree.List[csstree.Node]) csstree.WalkAction {
			prelude, err := csstree.Generate(node.(*csstree.Rule).Prelude, csstree.GenerateOptions{})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("rule", prelude)
			switch prelude {
			case "a":
				return csstree.WalkSkip
			case "b":
				return csstree.WalkBreak
			}
			return csstree.WalkContinue
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// rule a
	// rule b
}

func ExampleFind() {
	ast, err := csstree.Parse(`a { color: red; width: 10px; height: 2em }`, csstree.ParseOptions{})
	if err != nil {
		log.Fatal(err)
	}

	isDimension := func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem[csstree.Node], _ *csstree.List[csstree.Node]) bool {
		_, ok := node.(*csstree.Dimension)
		return ok
	}
	first := csstree.Find(ast, isDimension).(*csstree.Dimension)
	fmt.Println("first:", first.Value+first.Unit)
	last := csstree.FindLast(ast, isDimension).(*csstree.Dimension)
	fmt.Println("last:", last.Value+last.Unit)
	fmt.Println("all:", len(csstree.FindAll(ast, isDimension)))

	// Output:
	// first: 10px
	// last: 2em
	// all: 2
}

// Variable is a node type added by ExampleFork: $name in a value.
type Variable struct {
	Loc  *csstree.Location
	Name string
}

// Type returns "Variable".
func (n *Variable) Type() string { return "Variable" }

// Location returns n.Loc.
func (n *Variable) Location() *csstree.Location { return n.Loc }

func ExampleFork() {
	syntax, err := csstree.Fork(csstree.ExtendFunc(func(config *csstree.Config) *csstree.Config {
		// Parse $name in values as a Variable node. Fork passes a copy
		// of the default config, so changing its recognizer leaves the
		// default syntax as it is.
		value := config.Scope["Value"]
		defaultNode := value.Node
		value.Node = func(p *csstree.Parser, context *csstree.SequenceContext) csstree.Node {
			if p.IsDelim('$', 0) {
				start := p.TokenStart
				p.Next()
				name := p.Consume(tokenizer.Ident)
				return &Variable{Loc: p.Location(start, p.TokenStart), Name: name}
			}
			return defaultNode(p, context)
		}

		// Describe the node type so the walker, generator and lexer know it.
		config.Node["Variable"] = csstree.NodeConfig{
			Name: "Variable",
			Structure: []csstree.StructureField{
				{Name: "name", Types: []csstree.FieldType{{Kind: csstree.FieldString}}},
			},
			Generate: func(g *csstree.Generator, node csstree.Node) {
				g.Token(tokenizer.Delim, "$")
				g.Token(tokenizer.Ident, node.(*Variable).Name)
			},
		}
		return config
	}))
	if err != nil {
		log.Fatal(err)
	}

	ast, err := syntax.Parse(`$gap 1px`, csstree.ParseOptions{Context: csstree.ContextValue})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ast.(*csstree.Value).Children.First().(*Variable).Name)

	css, err := syntax.Generate(ast, csstree.GenerateOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(css)

	// The default syntax does not know $name.
	_, err = csstree.Parse(`$gap 1px`, csstree.ParseOptions{Context: csstree.ContextValue})
	fmt.Println(err)

	// Output:
	// gap
	// $gap 1px
	// 1:1: Unexpected input
}

func ExampleLexer_MatchPropertyString() {
	lexer := csstree.DefaultLexer()

	result, err := lexer.MatchPropertyString("border", "1px solid red")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("matched:", result.Matched != nil)

	_, err = lexer.MatchPropertyString("border", "1px solid 2px")
	var mismatch *csstree.SyntaxMatchError
	if errors.As(err, &mismatch) {
		fmt.Printf("mismatch at %q (offset %d) in %q\n",
			mismatch.CSS[mismatch.MismatchOffset:mismatch.MismatchOffset+mismatch.MismatchLength],
			mismatch.MismatchOffset, mismatch.CSS)
	}

	_, err = lexer.MatchPropertyString("colour", "red")
	var unknown *csstree.SyntaxReferenceError
	if errors.As(err, &unknown) {
		fmt.Println(unknown.Kind, unknown.Reference)
	}

	// Output:
	// matched: true
	// mismatch at "2px" (offset 10) in "1px solid 2px"
	// Property colour
}

func ExampleLexer_FindValueFragments() {
	value, err := csstree.Parse(`10px Arial, "Courier New", serif`, csstree.ParseOptions{Context: csstree.ContextValue})
	if err != nil {
		log.Fatal(err)
	}

	fragments, err := csstree.DefaultLexer().FindValueFragments("font", value, csstree.SyntaxRef{
		Kind: csstree.KindType,
		Name: "family-name",
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, fragment := range fragments {
		css, err := csstree.Generate(&csstree.Value{Children: fragment.Nodes}, csstree.GenerateOptions{})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(css)
	}

	// Output:
	// Arial
	// "Courier New"
}

func ExampleList() {
	var list csstree.List[string]
	list.AppendData("b").AppendData("c").PrependData("a")

	fmt.Println(slices.Collect(list.All()))
	fmt.Println(slices.Collect(list.Backward()))

	item := csstree.NewListItem("d")
	list.Append(item)
	fmt.Println(list.Len(), list.Last())

	// Output:
	// [a b c]
	// [c b a]
	// 4 d
}
