// Ported from lib/syntax/node/Identifier.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Identifier is an identifier.
type Identifier struct {
	Loc  *Location
	Name string
}

func (n *Identifier) Type() string        { return "Identifier" }
func (n *Identifier) Location() *Location { return n.Loc }

func (n *Identifier) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"name", n.Name},
	)
}

func identifierNodeType() NodeConfig {
	return NodeConfig{
		Name: "Identifier",
		Structure: []StructureField{
			{Name: "name", Types: []FieldType{stringField()}},
		},
		Generate: generateIdentifier,
	}
}

// Identifier parses an ident token.
func (p *Parser) Identifier() *Identifier {
	loc := p.GetLocation(p.TokenStart, p.TokenEnd)
	return &Identifier{
		Loc:  loc,
		Name: p.Consume(tokenizer.Ident),
	}
}

func generateIdentifier(g *Generator, node Node) {
	n := node.(*Identifier)
	g.Token(tokenizer.Ident, n.Name)
}
