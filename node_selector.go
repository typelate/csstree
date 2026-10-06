// Ported from lib/syntax/node/Selector.js.

package csstree

// Selector is a complex selector: compound selectors and combinators.
type Selector struct {
	Loc      *Location
	Children *List[Node]
}

func (n *Selector) Type() string        { return "Selector" }
func (n *Selector) Location() *Location { return n.Loc }

func (n *Selector) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"children", n.Children},
	)
}

func selectorNodeType() NodeConfig {
	return NodeConfig{
		Name: "Selector",
		Structure: []StructureField{
			{Name: "children", Types: []FieldType{listField(
				"TypeSelector",
				"IdSelector",
				"ClassSelector",
				"AttributeSelector",
				"PseudoClassSelector",
				"PseudoElementSelector",
				"Combinator",
			)}},
		},
		Generate: generateSelector,
	}
}

// Selector parses a selector with the Selector scope.
func (p *Parser) Selector() *Selector {
	children := p.ReadSequence(p.Config.Scope["Selector"])

	// nothing were consumed
	if p.GetFirstListNode(children) == nil {
		p.Error("Selector is expected", -1)
	}

	return &Selector{
		Loc:      p.GetLocationFromList(children),
		Children: children,
	}
}

func generateSelector(g *Generator, node Node) {
	n := node.(*Selector)
	g.Children(n.Children, nil)
}
