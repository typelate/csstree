// Ported from lib/syntax/node/String.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// String is a string; Value is decoded (no quotes, escapes resolved).
//
//csstree:node
type String struct {
	Loc   *Location
	Value string `css:"value,string"`
}

// StringNode parses a string token into a String node (named so as not to
// read as fmt.Stringer).
func (p *Parser) StringNode() *String {
	loc := p.Location(p.TokenStart, p.TokenEnd)
	return &String{
		Loc:   loc,
		Value: DecodeString(p.Consume(tokenizer.String)),
	}
}

func generateString(g *Generator, node Node) {
	n := node.(*String)
	g.Token(tokenizer.String, EncodeString(n.Value, false))
}
