// Ported from lib/syntax/node/Block.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func blockConsumeRaw(p *Parser) Node {
	return p.Raw(nil, true)
}

func blockConsumeRule(p *Parser) Node {
	return p.ParseWithFallback(func() Node { return p.Rule() }, func() Node { return blockConsumeRaw(p) })
}

func blockConsumeRawDeclaration(p *Parser) Node {
	return p.Raw(p.ConsumeUntilSemicolonIncluded, true)
}

func blockConsumeDeclaration(p *Parser) Node {
	if p.TokenType == tokenizer.Semicolon {
		return blockConsumeRawDeclaration(p)
	}

	node := p.ParseWithFallback(func() Node { return p.Declaration() }, func() Node { return blockConsumeRawDeclaration(p) })

	if p.TokenType == tokenizer.Semicolon {
		p.Next()
	}

	return node
}

// Block is a {}-block of rules or declarations.
type Block struct {
	Loc      *Location
	Children *List[Node]
}

func (n *Block) Type() string        { return "Block" }
func (n *Block) Location() *Location { return n.Loc }

func (n *Block) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"children", n.Children},
	)
}

func blockNodeType() NodeConfig {
	return NodeConfig{
		Name:        "Block",
		WalkContext: "block",
		Structure: []StructureField{
			{Name: "children", Types: []FieldType{listField(
				"Atrule",
				"Rule",
				"Declaration",
			)}},
		},
		Generate:        generateBlock,
		Walk:            walkBlock,
		Clone:           cloneBlock,
		FromPlainObject: fromPlainObjectBlock,
	}
}

func walkBlock(node Node, _ bool, _ func(Node) bool, walkList func(*List[Node]) bool) bool {
	n := node.(*Block)
	return n.Children != nil && walkList(n.Children)
}

func cloneBlock(node Node, clone func(Node) Node) Node {
	c := *node.(*Block)
	c.Loc = cloneLocation(c.Loc)
	c.Children = cloneList(c.Children, clone)
	return &c
}

func fromPlainObjectBlock(r *PlainObjectReader, object map[string]any) Node {
	return &Block{
		Loc:      r.Location(object),
		Children: r.List(object, "children"),
	}
}

// Block parses a {}-block; isStyleBlock makes it a block of declarations
// (and nested rules) instead of a block of rules.
func (p *Parser) Block(isStyleBlock bool) *Block {
	consumer := blockConsumeRule
	if isStyleBlock {
		consumer = blockConsumeDeclaration
	}
	start := p.TokenStart
	children := p.CreateList()

	p.Eat(tokenizer.LeftCurlyBracket)

scan:
	for !p.EOF {
		switch p.TokenType {
		case tokenizer.RightCurlyBracket:
			break scan

		case tokenizer.WhiteSpace, tokenizer.Comment:
			p.Next()

		case tokenizer.AtKeyword:
			children.Push(p.ParseWithFallback(func() Node { return p.Atrule(isStyleBlock) }, func() Node { return blockConsumeRaw(p) }))

		default:
			if isStyleBlock && p.IsDelim(ampersand, 0) {
				children.Push(blockConsumeRule(p))
			} else {
				children.Push(consumer(p))
			}
		}
	}

	if !p.EOF {
		p.Eat(tokenizer.RightCurlyBracket)
	}

	return &Block{
		Loc:      p.GetLocation(start, p.TokenStart),
		Children: children,
	}
}

func generateBlock(g *Generator, node Node) {
	n := node.(*Block)
	g.Token(tokenizer.LeftCurlyBracket, "{")
	g.Children(n.Children, func(prev Node) {
		if prev.Type() == "Declaration" {
			g.Token(tokenizer.Semicolon, ";")
		}
	})
	g.Token(tokenizer.RightCurlyBracket, "}")
}
