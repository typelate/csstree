// Ported from lib/syntax/node/Function.js.

package csstree

import (
	"github.com/typelate/csstree/tokenizer"
)

// Function is a function in a value: <function-token> <sequence> ).
//
//csstree:node walkContext=function
type Function struct {
	Loc      *Location
	Name     string      `css:"name,string"`
	Children *List[Node] `css:"children,[]"`
}

// Function parses <function-token> <sequence> ). The arguments are parsed
// by recognizer.Functions[name] when the recognizer has a parser for the
// function, else by readSequence(recognizer).
func (p *Parser) Function(readSequence func(recognizer *Recognizer) *List[Node], recognizer *Recognizer) *Function {
	start := p.TokenStart
	name := p.ConsumeFunctionName()
	nameLowerCase := toLowerCase(name)
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
		Loc:      p.Location(start, p.TokenStart),
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
