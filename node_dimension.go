// Ported from lib/syntax/node/Dimension.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Dimension is a number with a unit.
type Dimension struct {
	Loc   *Location
	Value string
	Unit  string
}

func (n *Dimension) Type() string        { return "Dimension" }
func (n *Dimension) Location() *Location { return n.Loc }

func (n *Dimension) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"value", n.Value},
		jsonField{"unit", n.Unit},
	)
}

func dimensionNodeType() NodeConfig {
	return NodeConfig{
		Name: "Dimension",
		Structure: []StructureField{
			{Name: "value", Types: []FieldType{stringField()}},
			{Name: "unit", Types: []FieldType{stringField()}},
		},
		Generate: generateDimension,
	}
}

// Dimension parses a dimension token.
func (p *Parser) Dimension() *Dimension {
	start := p.TokenStart
	value := p.ConsumeNumber(tokenizer.Dimension)

	return &Dimension{
		Loc:   p.GetLocation(start, p.TokenStart),
		Value: value,
		Unit:  p.Substring(start+len(value), p.TokenStart),
	}
}

func generateDimension(g *Generator, node Node) {
	n := node.(*Dimension)
	g.Token(tokenizer.Dimension, n.Value+n.Unit)
}
