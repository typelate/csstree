// Ported from lib/syntax/node/Brackets.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Brackets is a [] block in a value.
//
//csstree:node
type Brackets struct {
	Loc      *Location
	Children *List[Node] `css:"children,[]"`
}

// Brackets parses a [] block, reading its content with readSequence and
// recognizer.
func (p *Parser) Brackets(readSequence func(recognizer *Recognizer) *List[Node], recognizer *Recognizer) *Brackets {
	start := p.TokenStart

	p.Eat(tokenizer.LeftSquareBracket)

	children := readSequence(recognizer)

	if !p.EOF {
		p.Eat(tokenizer.RightSquareBracket)
	}

	return &Brackets{
		Loc:      p.Location(start, p.TokenStart),
		Children: children,
	}
}

func generateBrackets(g *Generator, node Node) {
	n := node.(*Brackets)
	g.Token(tokenizer.Delim, "[")
	g.Children(n.Children, nil)
	g.Token(tokenizer.Delim, "]")
}
