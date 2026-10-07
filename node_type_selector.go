// Ported from lib/syntax/node/TypeSelector.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func eatIdentifierOrAsterisk(p *Parser) {
	if p.TokenType != tokenizer.Ident &&
		!p.IsDelim(asterisk, 0) {
		p.Fail("Identifier or asterisk is expected")
	}

	p.Next()
}

// TypeSelector is a type (element) selector, with an optional namespace.
//
//csstree:node
type TypeSelector struct {
	Loc  *Location
	Name string `css:"name,string"`
}

// TypeSelector parses
//
//	ident
//	ident|ident
//	ident|*
//	*
//	*|ident
//	*|*
//	|ident
//	|*
func (p *Parser) TypeSelector() *TypeSelector {
	start := p.TokenStart

	if p.IsDelim(verticalLine, 0) {
		p.Next()
		eatIdentifierOrAsterisk(p)
	} else {
		eatIdentifierOrAsterisk(p)

		// a|| is a type selector followed by a column combinator
		if p.IsDelim(verticalLine, 0) && !p.IsDelim(verticalLine, 1) {
			p.Next()
			eatIdentifierOrAsterisk(p)
		}
	}

	return &TypeSelector{
		Loc:  p.Location(start, p.TokenStart),
		Name: p.SubstrToCursor(start),
	}
}

func generateTypeSelector(g *Generator, node Node) {
	n := node.(*TypeSelector)
	g.Tokenize(n.Name)
}
