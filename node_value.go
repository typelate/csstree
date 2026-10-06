// Ported from lib/syntax/node/Value.js.

package csstree

// Value is a declaration value.
type Value struct {
	Loc      *Location
	Children *List[Node]
}

func (n *Value) Type() string        { return "Value" }
func (n *Value) Location() *Location { return n.Loc }

func (n *Value) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"children", n.Children},
	)
}

func valueNodeType() NodeConfig {
	return NodeConfig{
		Name: "Value",
		Structure: []StructureField{
			{Name: "children", Types: []FieldType{listField()}},
		},
		Generate: generateValue,
	}
}

// Value parses a value with the Value scope.
func (p *Parser) Value() *Value {
	start := p.TokenStart
	children := p.ReadSequence(p.Config.Scope["Value"])

	return &Value{
		Loc:      p.GetLocation(start, p.TokenStart),
		Children: children,
	}
}

func generateValue(g *Generator, node Node) {
	n := node.(*Value)
	g.Children(n.Children, nil)
}
