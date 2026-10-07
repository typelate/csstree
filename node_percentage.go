// Ported from lib/syntax/node/Percentage.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Percentage is a percentage; Value excludes the % sign.
//
//csstree:node
type Percentage struct {
	Loc   *Location
	Value string `css:"value,string"`
}

// Percentage parses a percentage token.
func (p *Parser) Percentage() *Percentage {
	loc := p.Location(p.TokenStart, p.TokenEnd)
	return &Percentage{
		Loc:   loc,
		Value: p.ConsumeNumber(tokenizer.Percentage),
	}
}

func generatePercentage(g *Generator, node Node) {
	n := node.(*Percentage)
	g.Token(tokenizer.Percentage, n.Value+"%")
}
