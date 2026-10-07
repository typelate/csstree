// Ported from lib/syntax/node/Layer.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Layer is a cascade layer name, e.g. a.b.
//
//csstree:node
type Layer struct {
	Loc  *Location
	Name string `css:"name,string"`
}

// Layer parses a layer name.
func (p *Parser) Layer() *Layer {
	tokenStart := p.TokenStart
	name := p.Consume(tokenizer.Ident)

	for p.IsDelim(fullStop, 0) {
		p.Eat(tokenizer.Delim)
		name += "." + p.Consume(tokenizer.Ident)
	}

	return &Layer{
		Loc:  p.Location(tokenStart, p.TokenStart),
		Name: name,
	}
}

func generateLayer(g *Generator, node Node) {
	n := node.(*Layer)
	g.Tokenize(n.Name)
}
