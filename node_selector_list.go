// Ported from lib/syntax/node/SelectorList.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// SelectorList is a comma separated list of selectors.
type SelectorList struct {
	Loc      *Location
	Children *List[Node]
}

func (n *SelectorList) Type() string        { return "SelectorList" }
func (n *SelectorList) Location() *Location { return n.Loc }

func (n *SelectorList) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"children", n.Children},
	)
}

func selectorListNodeType() NodeConfig {
	return NodeConfig{
		Name:        "SelectorList",
		WalkContext: "selector",
		Structure: []StructureField{
			{Name: "children", Types: []FieldType{listField(
				"Selector",
				"Raw",
			)}},
		},
		Generate: generateSelectorList,
	}
}

// SelectorList parses a list of selectors.
func (p *Parser) SelectorList() *SelectorList {
	children := p.CreateList()

	for !p.EOF {
		children.Push(p.Selector())

		if p.TokenType == tokenizer.Comma {
			p.Next()
			continue
		}

		break
	}

	return &SelectorList{
		Loc:      p.GetLocationFromList(children),
		Children: children,
	}
}

func generateSelectorList(g *Generator, node Node) {
	n := node.(*SelectorList)
	g.Children(n.Children, func(Node) { g.Token(tokenizer.Comma, ",") })
}
