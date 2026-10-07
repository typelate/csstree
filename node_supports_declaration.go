// Ported from lib/syntax/node/SupportsDeclaration.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// SupportsDeclaration is a parenthesized declaration in a supports
// condition.
//
//csstree:node
type SupportsDeclaration struct {
	Loc         *Location
	Declaration *Declaration `css:"declaration,Declaration"`
}

// SupportsDeclaration parses ( <declaration> ).
func (p *Parser) SupportsDeclaration() *SupportsDeclaration {
	start := p.TokenStart

	p.Eat(tokenizer.LeftParenthesis)
	p.SkipSC()

	declaration := p.Declaration()

	if !p.EOF {
		p.Eat(tokenizer.RightParenthesis)
	}

	return &SupportsDeclaration{
		Loc:         p.Location(start, p.TokenStart),
		Declaration: declaration,
	}
}

func generateSupportsDeclaration(g *Generator, node Node) {
	n := node.(*SupportsDeclaration)
	g.Token(tokenizer.LeftParenthesis, "(")
	g.Node(nodeOrNil(n.Declaration))
	g.Token(tokenizer.RightParenthesis, ")")
}
