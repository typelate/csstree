// Ported from lib/syntax/node/Number.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Number is a number.
type Number struct {
	Loc   *Location
	Value string
}

func (n *Number) Type() string        { return "Number" }
func (n *Number) Location() *Location { return n.Loc }

func (n *Number) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"value", n.Value},
	)
}

func numberNodeType() NodeConfig {
	return NodeConfig{
		Name: "Number",
		Structure: []StructureField{
			{Name: "value", Types: []FieldType{stringField()}},
		},
		Generate: generateNumber,
	}
}

// Number parses a number token.
func (p *Parser) Number() *Number {
	loc := p.GetLocation(p.TokenStart, p.TokenEnd)
	return &Number{
		Loc:   loc,
		Value: p.Consume(tokenizer.Number),
	}
}

func generateNumber(g *Generator, node Node) {
	n := node.(*Number)
	g.Token(tokenizer.Number, n.Value)
}
