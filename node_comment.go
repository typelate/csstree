// Ported from lib/syntax/node/Comment.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Comment is a /* */ comment kept in the AST (exclamation comments at the
// top level of a style sheet).
//
//csstree:node
type Comment struct {
	Loc   *Location
	Value string `css:"value,string"`
}

// Comment parses a comment token.
func (p *Parser) Comment() *Comment {
	start := p.TokenStart
	end := p.TokenEnd

	p.Eat(tokenizer.Comment)

	if (end-start+2) >= 2 &&
		p.CharCodeAt(end-2) == asterisk &&
		p.CharCodeAt(end-1) == solidus {
		end -= 2
	}

	return &Comment{
		Loc:   p.Location(start, p.TokenStart),
		Value: p.Substring(start+2, end),
	}
}

func generateComment(g *Generator, node Node) {
	n := node.(*Comment)
	g.Token(tokenizer.Comment, "/*"+n.Value+"*/")
}
