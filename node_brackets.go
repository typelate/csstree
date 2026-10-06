// Ported from lib/syntax/node/Brackets.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Brackets is a [] block in a value.
type Brackets struct {
	Loc      *Location
	Children *List[Node]
}

func (n *Brackets) Type() string        { return "Brackets" }
func (n *Brackets) Location() *Location { return n.Loc }

func (n *Brackets) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"children", n.Children},
	)
}

func bracketsNodeType() NodeConfig {
	return NodeConfig{
		Name: "Brackets",
		Structure: []StructureField{
			{Name: "children", Types: []FieldType{listField()}},
		},
		Generate: generateBrackets,
	}
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
		Loc:      p.GetLocation(start, p.TokenStart),
		Children: children,
	}
}

func generateBrackets(g *Generator, node Node) {
	n := node.(*Brackets)
	g.Token(tokenizer.Delim, "[")
	g.Children(n.Children, nil)
	g.Token(tokenizer.Delim, "]")
}
