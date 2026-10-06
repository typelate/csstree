// Ported from lib/syntax/node/LayerList.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// LayerList is a comma separated list of layer names.
type LayerList struct {
	Loc      *Location
	Children *List[Node]
}

func (n *LayerList) Type() string        { return "LayerList" }
func (n *LayerList) Location() *Location { return n.Loc }

func (n *LayerList) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"children", n.Children},
	)
}

func layerListNodeType() NodeConfig {
	return NodeConfig{
		Name: "LayerList",
		Structure: []StructureField{
			{Name: "children", Types: []FieldType{listField(
				"Layer",
			)}},
		},
		Generate: generateLayerList,
	}
}

// LayerList parses a list of layer names.
func (p *Parser) LayerList() *LayerList {
	children := p.CreateList()

	p.SkipSC()

	for !p.EOF {
		children.Push(p.Layer())

		if p.LookupTypeNonSC(0) != tokenizer.Comma {
			break
		}

		p.SkipSC()
		p.Next()
		p.SkipSC()
	}

	return &LayerList{
		Loc:      p.GetLocationFromList(children),
		Children: children,
	}
}

func generateLayerList(g *Generator, node Node) {
	n := node.(*LayerList)
	g.Children(n.Children, func(Node) { g.Token(tokenizer.Comma, ",") })
}
