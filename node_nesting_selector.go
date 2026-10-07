// Ported from lib/syntax/node/NestingSelector.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// NestingSelector is the & selector.
//
//csstree:node
type NestingSelector struct {
	Loc *Location
}

// NestingSelector parses &.
func (p *Parser) NestingSelector() *NestingSelector {
	start := p.TokenStart

	p.EatDelim(ampersand)

	return &NestingSelector{
		Loc: p.Location(start, p.TokenStart),
	}
}

func generateNestingSelector(g *Generator, _ Node) {
	g.Token(tokenizer.Delim, "&")
}
