// Ported from lib/syntax/node/CDO.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// CDO is a <!-- token at the top level of a style sheet.
type CDO struct {
	Loc *Location
}

func (n *CDO) Type() string        { return "CDO" }
func (n *CDO) Location() *Location { return n.Loc }

func (n *CDO) MarshalJSON() ([]byte, error) {
	return marshalNode(n)
}

func cdoNodeType() NodeConfig {
	return NodeConfig{
		Name:      "CDO",
		Structure: []StructureField{},
		Generate:  generateCDO,
	}
}

// CDO parses a <!-- token.
func (p *Parser) CDO() *CDO {
	start := p.TokenStart

	p.Eat(tokenizer.CDO) // <!--

	return &CDO{
		Loc: p.GetLocation(start, p.TokenStart),
	}
}

func generateCDO(g *Generator, _ Node) {
	g.Token(tokenizer.CDO, "<!--")
}
