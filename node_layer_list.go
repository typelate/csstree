// Ported from lib/syntax/node/LayerList.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// LayerList is a comma separated list of layer names.
//
//csstree:node
type LayerList struct {
	Loc      *Location
	Children *List[Node] `css:"children,[Layer]"`
}

// LayerList parses a list of layer names.
func (p *Parser) LayerList() *LayerList {
	children := new(List[Node])

	p.SkipSC()

	for !p.EOF {
		children.AppendData(p.Layer())

		if p.LookupTypeNonSC(0) != tokenizer.Comma {
			break
		}

		p.SkipSC()
		p.Next()
		p.SkipSC()
	}

	return &LayerList{
		Loc:      p.LocationFromList(children),
		Children: children,
	}
}

func generateLayerList(g *Generator, node Node) {
	n := node.(*LayerList)
	g.Children(n.Children, func(Node) { g.Token(tokenizer.Comma, ",") })
}
