// Ported from lib/syntax/node/CDC.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// CDC is a --> token at the top level of a style sheet.
//
//csstree:node
type CDC struct {
	Loc *Location
}

// CDC parses a --> token.
func (p *Parser) CDC() *CDC {
	start := p.TokenStart

	p.Eat(tokenizer.CDC) // -->

	return &CDC{
		Loc: p.Location(start, p.TokenStart),
	}
}

func generateCDC(g *Generator, _ Node) {
	g.Token(tokenizer.CDC, "-->")
}
