// Ported from lib/syntax/node/CDO.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// CDO is a <!-- token at the top level of a style sheet.
//
//csstree:node
type CDO struct {
	Loc *Location
}

// CDO parses a <!-- token.
func (p *Parser) CDO() *CDO {
	start := p.TokenStart

	p.Eat(tokenizer.CDO) // <!--

	return &CDO{
		Loc: p.Location(start, p.TokenStart),
	}
}

func generateCDO(g *Generator, _ Node) {
	g.Token(tokenizer.CDO, "<!--")
}
