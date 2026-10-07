// Ported from lib/syntax/node/Parentheses.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Parentheses is a () block in a value.
//
//csstree:node
type Parentheses struct {
	Loc      *Location
	Children *List[Node] `css:"children,[]"`
}

// Parentheses parses a () block, reading its content with readSequence and
// recognizer.
func (p *Parser) Parentheses(readSequence func(recognizer *Recognizer) *List[Node], recognizer *Recognizer) *Parentheses {
	start := p.TokenStart

	p.Eat(tokenizer.LeftParenthesis)

	children := readSequence(recognizer)

	if !p.EOF {
		p.Eat(tokenizer.RightParenthesis)
	}

	return &Parentheses{
		Loc:      p.Location(start, p.TokenStart),
		Children: children,
	}
}

func generateParentheses(g *Generator, node Node) {
	n := node.(*Parentheses)
	g.Token(tokenizer.LeftParenthesis, "(")
	g.Children(n.Children, nil)
	g.Token(tokenizer.RightParenthesis, ")")
}
