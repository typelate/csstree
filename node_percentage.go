// Ported from lib/syntax/node/Percentage.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Percentage is a percentage; Value excludes the % sign.
type Percentage struct {
	Loc   *Location
	Value string
}

func (n *Percentage) Type() string        { return "Percentage" }
func (n *Percentage) Location() *Location { return n.Loc }

func (n *Percentage) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"value", n.Value},
	)
}

func percentageNodeType() NodeConfig {
	return NodeConfig{
		Name: "Percentage",
		Structure: []StructureField{
			{Name: "value", Types: []FieldType{stringField()}},
		},
		Generate: generatePercentage,
	}
}

// Percentage parses a percentage token.
func (p *Parser) Percentage() *Percentage {
	loc := p.GetLocation(p.TokenStart, p.TokenEnd)
	return &Percentage{
		Loc:   loc,
		Value: p.ConsumeNumber(tokenizer.Percentage),
	}
}

func generatePercentage(g *Generator, node Node) {
	n := node.(*Percentage)
	g.Token(tokenizer.Percentage, n.Value+"%")
}
