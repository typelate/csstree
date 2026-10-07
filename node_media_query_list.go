// Ported from lib/syntax/node/MediaQueryList.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// MediaQueryList is a comma separated list of media queries.
//
//csstree:node
type MediaQueryList struct {
	Loc      *Location
	Children *List[Node] `css:"children,[MediaQuery]"`
}

// MediaQueryList parses a list of media queries.
func (p *Parser) MediaQueryList() *MediaQueryList {
	children := new(List[Node])

	p.SkipSC()

	for !p.EOF {
		children.AppendData(p.MediaQuery())

		if p.TokenType != tokenizer.Comma {
			break
		}

		p.Next()
	}

	return &MediaQueryList{
		Loc:      p.LocationFromList(children),
		Children: children,
	}
}

func generateMediaQueryList(g *Generator, node Node) {
	n := node.(*MediaQueryList)
	g.Children(n.Children, func(Node) { g.Token(tokenizer.Comma, ",") })
}
