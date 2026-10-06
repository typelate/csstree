// Ported from lib/syntax/node/String.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// String is a string; Value is decoded (no quotes, escapes resolved).
type String struct {
	Loc   *Location
	Value string
}

func (n *String) Type() string        { return "String" }
func (n *String) Location() *Location { return n.Loc }

func (n *String) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"value", n.Value},
	)
}

func stringNodeType() NodeConfig {
	return NodeConfig{
		Name: "String",
		Structure: []StructureField{
			{Name: "value", Types: []FieldType{stringField()}},
		},
		Generate: generateString,
	}
}

// String parses a string token.
func (p *Parser) String() *String {
	loc := p.GetLocation(p.TokenStart, p.TokenEnd)
	return &String{
		Loc:   loc,
		Value: DecodeString(p.Consume(tokenizer.String)),
	}
}

func generateString(g *Generator, node Node) {
	n := node.(*String)
	g.Token(tokenizer.String, EncodeString(n.Value, false))
}
