// Ported from lib/syntax/node/Value.js.

package csstree

// Value is a declaration value.
//
//csstree:node
type Value struct {
	Loc      *Location
	Children *List[Node] `css:"children,[]"`
}

// Value parses a value with the Value scope.
func (p *Parser) Value() *Value {
	start := p.TokenStart
	children := p.ReadSequence(p.ScopeRecognizer("Value"))

	return &Value{
		Loc:      p.Location(start, p.TokenStart),
		Children: children,
	}
}

func generateValue(g *Generator, node Node) {
	n := node.(*Value)
	g.Children(n.Children, nil)
}
