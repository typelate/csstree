// Ported from lib/syntax/node/GeneralEnclosed.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// GeneralEnclosed is a condition term the parser does not know:
//
//	<function-token> <any-value> )
//	( <any-value> )
//
//csstree:node
type GeneralEnclosed struct {
	Loc  *Location
	Kind string `css:"kind,string"`
	// Function is the function name; "" (upstream null) for ( <any-value> ).
	Function string      `css:"function,string|null"`
	Children *List[Node] `css:"children,[]"`
}

// GeneralEnclosed parses a general enclosed term of a condition of kind.
func (p *Parser) GeneralEnclosed(kind string) *GeneralEnclosed {
	start := p.TokenStart
	functionName := ""

	if p.TokenType == tokenizer.Function {
		functionName = p.ConsumeFunctionName()
	} else {
		p.Eat(tokenizer.LeftParenthesis)
	}

	var children *List[Node]
	p.ParseWithFallback(
		func() Node {
			startValueToken := p.TokenIndex
			children = p.ReadSequence(p.ScopeRecognizer("Value"))

			if !p.EOF &&
				!p.IsBalanceEdge(startValueToken) {
				p.Unexpected()
			}

			return nil
		},
		func() Node {
			children = new(List[Node]).AppendData(
				p.Raw(nil, false),
			)
			return children.First()
		},
	)

	if !p.EOF {
		p.Eat(tokenizer.RightParenthesis)
	}

	return &GeneralEnclosed{
		Loc:      p.Location(start, p.TokenStart),
		Kind:     kind,
		Function: functionName,
		Children: children,
	}
}

func generateGeneralEnclosed(g *Generator, node Node) {
	n := node.(*GeneralEnclosed)
	if n.Function != "" {
		g.Token(tokenizer.Function, n.Function+"(")
	} else {
		g.Token(tokenizer.LeftParenthesis, "(")
	}

	g.Children(n.Children, nil)
	g.Token(tokenizer.RightParenthesis, ")")
}
