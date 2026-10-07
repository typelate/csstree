// Ported from lib/syntax/node/Operator.js.

package csstree

// Operator is '/' | '*' | ',' | ':' | '+' | '-'
//
//csstree:node
type Operator struct {
	Loc   *Location
	Value string `css:"value,string"`
}

// Operator parses the current token as an operator.
func (p *Parser) Operator() *Operator {
	start := p.TokenStart

	p.Next()

	return &Operator{
		Loc:   p.Location(start, p.TokenStart),
		Value: p.SubstrToCursor(start),
	}
}

func generateOperator(g *Generator, node Node) {
	n := node.(*Operator)
	g.Tokenize(n.Value)
}
