// Ported from lib/syntax/node/DeclarationList.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func declarationListConsumeRaw(p *Parser) Node {
	return p.Raw(p.ConsumeUntilSemicolonIncluded, true)
}

// DeclarationList is a list of declarations without braces, e.g. the
// content of a style attribute.
type DeclarationList struct {
	Loc      *Location
	Children *List[Node]
}

func (n *DeclarationList) Type() string        { return "DeclarationList" }
func (n *DeclarationList) Location() *Location { return n.Loc }

func (n *DeclarationList) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"children", n.Children},
	)
}

func declarationListNodeType() NodeConfig {
	return NodeConfig{
		Name: "DeclarationList",
		Structure: []StructureField{
			{Name: "children", Types: []FieldType{listField(
				"Declaration",
				"Atrule",
				"Rule",
			)}},
		},
		Generate: generateDeclarationList,
	}
}

// DeclarationList parses a list of declarations.
func (p *Parser) DeclarationList() *DeclarationList {
	children := p.CreateList()

	for !p.EOF {
		switch p.TokenType {
		case tokenizer.WhiteSpace, tokenizer.Comment, tokenizer.Semicolon:
			p.Next()

		case tokenizer.AtKeyword:
			children.Push(p.ParseWithFallback(func() Node { return p.Atrule(true) }, func() Node { return declarationListConsumeRaw(p) }))

		default:
			if p.IsDelim(ampersand, 0) {
				children.Push(p.ParseWithFallback(func() Node { return p.Rule() }, func() Node { return declarationListConsumeRaw(p) }))
			} else {
				children.Push(p.ParseWithFallback(func() Node { return p.Declaration() }, func() Node { return declarationListConsumeRaw(p) }))
			}
		}
	}

	return &DeclarationList{
		Loc:      p.GetLocationFromList(children),
		Children: children,
	}
}

func generateDeclarationList(g *Generator, node Node) {
	n := node.(*DeclarationList)
	g.Children(n.Children, func(prev Node) {
		if prev.Type() == "Declaration" {
			g.Token(tokenizer.Semicolon, ";")
		}
	})
}
