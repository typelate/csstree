// Ported from lib/syntax/node/SupportsDeclaration.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// SupportsDeclaration is a parenthesized declaration in a supports
// condition.
type SupportsDeclaration struct {
	Loc         *Location
	Declaration *Declaration
}

func (n *SupportsDeclaration) Type() string        { return "SupportsDeclaration" }
func (n *SupportsDeclaration) Location() *Location { return n.Loc }

func (n *SupportsDeclaration) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"declaration", n.Declaration},
	)
}

func supportsDeclarationNodeType() NodeConfig {
	return NodeConfig{
		Name: "SupportsDeclaration",
		Structure: []StructureField{
			{Name: "declaration", Types: []FieldType{nodeField("Declaration")}},
		},
		Generate: generateSupportsDeclaration,
	}
}

// SupportsDeclaration parses ( <declaration> ).
func (p *Parser) SupportsDeclaration() *SupportsDeclaration {
	start := p.TokenStart

	p.Eat(tokenizer.LeftParenthesis)
	p.SkipSC()

	declaration := p.Declaration()

	if !p.EOF {
		p.Eat(tokenizer.RightParenthesis)
	}

	return &SupportsDeclaration{
		Loc:         p.GetLocation(start, p.TokenStart),
		Declaration: declaration,
	}
}

func generateSupportsDeclaration(g *Generator, node Node) {
	n := node.(*SupportsDeclaration)
	g.Token(tokenizer.LeftParenthesis, "(")
	g.Node(n.Declaration)
	g.Token(tokenizer.RightParenthesis, ")")
}
