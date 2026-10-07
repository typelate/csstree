// Ported from lib/syntax/node/ClassSelector.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// ClassSelector is '.' ident.
//
//csstree:node
type ClassSelector struct {
	Loc  *Location
	Name string `css:"name,string"`
}

// ClassSelector parses a class selector.
func (p *Parser) ClassSelector() *ClassSelector {
	p.EatDelim(fullStop)

	loc := p.Location(p.TokenStart-1, p.TokenEnd)
	return &ClassSelector{
		Loc:  loc,
		Name: p.Consume(tokenizer.Ident),
	}
}

func generateClassSelector(g *Generator, node Node) {
	n := node.(*ClassSelector)
	g.Token(tokenizer.Delim, ".")
	g.Token(tokenizer.Ident, n.Name)
}
