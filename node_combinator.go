// Ported from lib/syntax/node/Combinator.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Combinator is a selector combinator: + | > | ~ | || | /deep/ or " " for
// descendant.
//
//csstree:node
type Combinator struct {
	Loc  *Location
	Name string `css:"name,string"`
}

// Combinator parses + | > | ~ | || | /deep/
func (p *Parser) Combinator() *Combinator {
	start := p.TokenStart
	var name string

	switch p.TokenType {
	case tokenizer.WhiteSpace:
		name = " "

	case tokenizer.Delim:
		switch p.CharCodeAt(p.TokenStart) {
		case greaterThanSign, plusSign, tilde:
			p.Next()

		case verticalLine:
			// column combinator: two adjacent | delims
			p.Next()
			if !p.IsDelim(verticalLine, 0) {
				p.Fail(`Delim "|" is expected`)
			}
			p.Next()

		case solidus:
			p.Next()
			p.EatIdent("deep")
			p.EatDelim(solidus)

		default:
			p.Fail("Combinator is expected")
		}

		name = p.SubstrToCursor(start)
	}

	return &Combinator{
		Loc:  p.Location(start, p.TokenStart),
		Name: name,
	}
}

func generateCombinator(g *Generator, node Node) {
	n := node.(*Combinator)
	g.Tokenize(n.Name)
}
