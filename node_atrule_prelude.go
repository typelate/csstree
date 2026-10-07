// Ported from lib/syntax/node/AtrulePrelude.js.

package csstree

import (
	"github.com/typelate/csstree/tokenizer"
)

// AtrulePrelude is the prelude of an at-rule, between its name and its
// block or semicolon.
//
//csstree:node walkContext=atrulePrelude
type AtrulePrelude struct {
	Loc      *Location
	Children *List[Node] `css:"children,[]"`
}

// AtrulePrelude parses the prelude of the at-rule name with the at-rule's
// prelude parser, or with the AtrulePrelude scope when the at-rule has none
// or name is "".
func (p *Parser) AtrulePrelude(name string) *AtrulePrelude {
	var children *List[Node]

	name = toLowerCase(name)

	p.SkipSC()

	if atrule, ok := p.config.Atrule[name]; ok &&
		atrule.Prelude != nil {
		// custom consumer
		children = atrule.Prelude(p)
	} else {
		// default consumer
		children = p.ReadSequence(p.ScopeRecognizer("AtrulePrelude"))
	}

	p.SkipSC()

	if !p.EOF &&
		p.TokenType != tokenizer.LeftCurlyBracket &&
		p.TokenType != tokenizer.Semicolon {
		p.Fail("Semicolon or block is expected")
	}

	return &AtrulePrelude{
		Loc:      p.LocationFromList(children),
		Children: children,
	}
}

func generateAtrulePrelude(g *Generator, node Node) {
	n := node.(*AtrulePrelude)
	g.Children(n.Children, nil)
}
