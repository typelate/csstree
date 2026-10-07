// Ported from lib/syntax/node/Number.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Number is a number.
//
//csstree:node
type Number struct {
	Loc   *Location
	Value string `css:"value,string"`
}

// Number parses a number token.
func (p *Parser) Number() *Number {
	loc := p.Location(p.TokenStart, p.TokenEnd)
	return &Number{
		Loc:   loc,
		Value: p.Consume(tokenizer.Number),
	}
}

func generateNumber(g *Generator, node Node) {
	n := node.(*Number)
	g.Token(tokenizer.Number, n.Value)
}
