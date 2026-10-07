// Ported from lib/syntax/node/Dimension.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Dimension is a number with a unit.
//
//csstree:node
type Dimension struct {
	Loc   *Location
	Value string `css:"value,string"`
	Unit  string `css:"unit,string"`
}

// Dimension parses a dimension token.
func (p *Parser) Dimension() *Dimension {
	start := p.TokenStart
	value := p.ConsumeNumber(tokenizer.Dimension)

	return &Dimension{
		Loc:   p.Location(start, p.TokenStart),
		Value: value,
		Unit:  p.Substring(start+len(value), p.TokenStart),
	}
}

func generateDimension(g *Generator, node Node) {
	n := node.(*Dimension)
	g.Token(tokenizer.Dimension, n.Value+n.Unit)
}
