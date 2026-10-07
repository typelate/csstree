// Ported from lib/syntax/node/Hash.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Hash is '#' ident in a value, e.g. a hex color.
//
//csstree:node
type Hash struct {
	Loc   *Location
	Value string `css:"value,string"`
}

// Hash parses a hash token.
func (p *Parser) Hash() *Hash {
	start := p.TokenStart

	p.Eat(tokenizer.Hash)

	return &Hash{
		Loc:   p.Location(start, p.TokenStart),
		Value: p.SubstrToCursor(start + 1),
	}
}

func generateHash(g *Generator, node Node) {
	n := node.(*Hash)
	g.Token(tokenizer.Hash, "#"+n.Value)
}
