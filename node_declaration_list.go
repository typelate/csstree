// Ported from lib/syntax/node/DeclarationList.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func declarationListConsumeRaw(p *Parser) Node {
	return p.Raw(p.ConsumeUntilSemicolonIncluded, true)
}

// DeclarationList is a list of declarations without braces, e.g. the
// content of a style attribute.
//
//csstree:node
type DeclarationList struct {
	Loc      *Location
	Children *List[Node] `css:"children,[Declaration|Atrule|Rule]"`
}

// DeclarationList parses a list of declarations.
func (p *Parser) DeclarationList() *DeclarationList {
	children := new(List[Node])

	for !p.EOF {
		switch p.TokenType {
		case tokenizer.WhiteSpace, tokenizer.Comment, tokenizer.Semicolon:
			p.Next()

		case tokenizer.AtKeyword:
			children.AppendData(p.ParseWithFallback(func() Node { return p.Atrule(true) }, func() Node { return declarationListConsumeRaw(p) }))

		default:
			if isNestedRuleAhead(p) {
				children.AppendData(p.ParseWithFallback(func() Node { return p.Rule() }, func() Node { return declarationListConsumeRaw(p) }))
			} else {
				children.AppendData(p.ParseWithFallback(func() Node { return p.Declaration() }, func() Node { return declarationListConsumeRaw(p) }))
			}
		}
	}

	return &DeclarationList{
		Loc:      p.LocationFromList(children),
		Children: children,
	}
}

func generateDeclarationList(g *Generator, node Node) {
	n := node.(*DeclarationList)
	g.Children(n.Children, func(prev Node) {
		if _, ok := prev.(*Declaration); ok {
			g.Token(tokenizer.Semicolon, ";")
		}
	})
}
