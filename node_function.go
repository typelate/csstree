// Ported from lib/syntax/node/Function.js.

package csstree

import (
	"strings"

	"github.com/typelate/csstree/tokenizer"
)

// Function is a function in a value: <function-token> <sequence> ).
type Function struct {
	Loc      *Location
	Name     string
	Children *List[Node]
}

func (n *Function) Type() string        { return "Function" }
func (n *Function) Location() *Location { return n.Loc }

func (n *Function) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"name", n.Name},
		jsonField{"children", n.Children},
	)
}

func functionNodeType() NodeConfig {
	return NodeConfig{
		Name:        "Function",
		WalkContext: "function",
		Structure: []StructureField{
			{Name: "name", Types: []FieldType{stringField()}},
			{Name: "children", Types: []FieldType{listField()}},
		},
		Generate: generateFunction,
	}
}

// Function parses <function-token> <sequence> ). The arguments are parsed
// by recognizer.Functions[name] when the recognizer has a parser for the
// function, else by readSequence(recognizer).
func (p *Parser) Function(readSequence func(recognizer *Recognizer) *List[Node], recognizer *Recognizer) *Function {
	start := p.TokenStart
	name := p.ConsumeFunctionName()
	nameLowerCase := strings.ToLower(name)
	var children *List[Node]

	if fn, ok := recognizer.Functions[nameLowerCase]; ok {
		children = fn(p, recognizer)
	} else {
		children = readSequence(recognizer)
	}

	if !p.EOF {
		p.Eat(tokenizer.RightParenthesis)
	}

	return &Function{
		Loc:      p.GetLocation(start, p.TokenStart),
		Name:     name,
		Children: children,
	}
}

func generateFunction(g *Generator, node Node) {
	n := node.(*Function)
	g.Token(tokenizer.Function, n.Name+"(")
	g.Children(n.Children, nil)
	g.Token(tokenizer.RightParenthesis, ")")
}
