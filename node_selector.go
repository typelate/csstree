// Ported from lib/syntax/node/Selector.js.

package csstree

// Selector is a complex selector: compound selectors and combinators.
//
//csstree:node
type Selector struct {
	Loc      *Location
	Children *List[Node] `css:"children,[TypeSelector|IdSelector|ClassSelector|AttributeSelector|PseudoClassSelector|PseudoElementSelector|Combinator]"`
}

// Selector parses a selector with the Selector scope.
func (p *Parser) Selector() *Selector {
	children := p.ReadSequence(p.ScopeRecognizer("Selector"))

	// nothing were consumed
	if children.IsEmpty() {
		p.Fail("Selector is expected")
	}

	return &Selector{
		Loc:      p.LocationFromList(children),
		Children: children,
	}
}

func generateSelector(g *Generator, node Node) {
	n := node.(*Selector)
	g.Children(n.Children, nil)
}
