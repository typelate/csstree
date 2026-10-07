// Ported from lib/syntax/node/WhiteSpace.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// WhiteSpace is whitespace kept in a value.
//
//csstree:node
type WhiteSpace struct {
	Loc   *Location
	Value string `css:"value,string"`
}

// WhiteSpace parses a whitespace token. Upstream returns one frozen shared
// node; the port returns a new node each time.
func (p *Parser) WhiteSpace() *WhiteSpace {
	p.Eat(tokenizer.WhiteSpace)
	return &WhiteSpace{
		Loc:   nil,
		Value: " ",
	}

	// return {
	//     type: 'WhiteSpace',
	//     loc: this.getLocation(this.tokenStart, this.tokenEnd),
	//     value: this.consume(WHITESPACE)
	// };
}

func generateWhiteSpace(g *Generator, node Node) {
	n := node.(*WhiteSpace)
	g.Token(tokenizer.WhiteSpace, n.Value)
}
