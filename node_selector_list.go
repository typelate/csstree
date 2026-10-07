// Ported from lib/syntax/node/SelectorList.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// SelectorList is a comma separated list of selectors.
//
//csstree:node walkContext=selector
type SelectorList struct {
	Loc      *Location
	Children *List[Node] `css:"children,[Selector|Raw]"`
}

// SelectorList parses a list of selectors.
func (p *Parser) SelectorList() *SelectorList {
	children := new(List[Node])

	for !p.EOF {
		children.AppendData(p.Selector())

		if p.TokenType == tokenizer.Comma {
			p.Next()
			continue
		}

		break
	}

	return &SelectorList{
		Loc:      p.LocationFromList(children),
		Children: children,
	}
}

func generateSelectorList(g *Generator, node Node) {
	n := node.(*SelectorList)
	g.Children(n.Children, func(Node) { g.Token(tokenizer.Comma, ",") })
}
