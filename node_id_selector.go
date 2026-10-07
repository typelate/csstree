// Ported from lib/syntax/node/IdSelector.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// IDSelector is an id selector, '#' ident. Its type name is "IdSelector",
// as upstream names it.
//
//csstree:node name=IdSelector
type IDSelector struct {
	Loc  *Location
	Name string `css:"name,string"`
}

// IDSelector parses an id selector.
func (p *Parser) IDSelector() *IDSelector {
	start := p.TokenStart

	// TODO: check value is an ident
	p.Eat(tokenizer.Hash)

	return &IDSelector{
		Loc:  p.Location(start, p.TokenStart),
		Name: p.SubstrToCursor(start + 1),
	}
}

func generateIDSelector(g *Generator, node Node) {
	n := node.(*IDSelector)
	// Using Delim instead of Hash is a hack to avoid for a whitespace between ident and id-selector
	// in safe mode (e.g. "a#id"), because IE11 doesn't allow a sequence <ident-token> <hash-token>
	// without a whitespace in values (e.g. "1px solid#000")
	g.Token(tokenizer.Delim, "#"+n.Name)
}
