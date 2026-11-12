// Ported from lib/syntax/node/PseudoElementSelector.js.

package csstree

import (
	"github.com/typelate/csstree/tokenizer"
)

// PseudoElementSelector is :: [ <ident> | <function-token> <any-value>? ) ].
//
//csstree:node walkContext=function
type PseudoElementSelector struct {
	Loc  *Location
	Name string `css:"name,string"`
	// Children holds the arguments of a functional pseudo-element; nil for
	// a pseudo-element written without parentheses.
	Children *List[Node] `css:"children,[Raw]|null"`
}

// PseudoElementSelector parses :: [ <ident> | <function-token> <any-value>? ) ]
func (p *Parser) PseudoElementSelector() *PseudoElementSelector {
	start := p.TokenStart
	var children *List[Node]
	var name string

	p.Eat(tokenizer.Colon)
	p.Eat(tokenizer.Colon)

	if p.TokenType == tokenizer.Function {
		name = p.ConsumeFunctionName()
		nameLowerCase := toLowerCase(name)

		if p.LookupNonWSType(0) == tokenizer.RightParenthesis {
			// (upstream leaves the whitespace and comments, and then fails
			// to find the right parenthesis)
			p.SkipSC()
			children = new(List[Node])
		} else if pseudo, ok := p.config.Pseudo[nameLowerCase]; ok {
			p.SkipSC()
			children = pseudo(p)
			p.SkipSC()
		} else {
			children = new(List[Node])
			children.AppendData(
				p.Raw(nil, false),
			)
		}

		p.Eat(tokenizer.RightParenthesis)
	} else {
		name = p.Consume(tokenizer.Ident)
	}

	return &PseudoElementSelector{
		Loc:      p.Location(start, p.TokenStart),
		Name:     name,
		Children: children,
	}
}

func generatePseudoElementSelector(g *Generator, node Node) {
	n := node.(*PseudoElementSelector)
	g.Token(tokenizer.Colon, ":")
	g.Token(tokenizer.Colon, ":")

	if n.Children == nil {
		g.Token(tokenizer.Ident, n.Name)
	} else {
		g.Token(tokenizer.Function, n.Name+"(")
		g.Children(n.Children, nil)
		g.Token(tokenizer.RightParenthesis, ")")
	}
}
