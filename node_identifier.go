// Ported from lib/syntax/node/Identifier.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Identifier is an identifier.
//
//csstree:node
type Identifier struct {
	Loc  *Location
	Name string `css:"name,string"`
}

// Identifier parses an ident token.
func (p *Parser) Identifier() *Identifier {
	loc := p.Location(p.TokenStart, p.TokenEnd)
	return &Identifier{
		Loc:  loc,
		Name: p.Consume(tokenizer.Ident),
	}
}

func generateIdentifier(g *Generator, node Node) {
	n := node.(*Identifier)
	g.Token(tokenizer.Ident, n.Name)
}
