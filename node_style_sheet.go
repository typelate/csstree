// Ported from lib/syntax/node/StyleSheet.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func styleSheetConsumeRaw(p *Parser) Node {
	return p.Raw(nil, false)
}

// StyleSheet is a style sheet.
//
//csstree:node walkContext=stylesheet
type StyleSheet struct {
	Loc      *Location
	Children *List[Node] `css:"children,[Comment|CDO|CDC|Atrule|Rule|Raw]"`
}

// StyleSheet parses a style sheet.
func (p *Parser) StyleSheet() *StyleSheet {
	start := p.TokenStart
	children := new(List[Node])
	var child Node

	for !p.EOF {
		switch p.TokenType {
		case tokenizer.WhiteSpace:
			p.Next()
			continue

		case tokenizer.Comment:
			// ignore comments except exclamation comments (i.e. /*! .. */) on top level
			if p.CharCodeAt(p.TokenStart+2) != exclamationMark {
				p.Next()
				continue
			}

			child = p.Comment()

		case tokenizer.CDO: // <!--
			child = p.CDO()

		case tokenizer.CDC: // -->
			child = p.CDC()

		// CSS Syntax Module Level 3
		// §2.2 Error handling
		// At the "top level" of a stylesheet, an <at-keyword-token> starts an at-rule.
		case tokenizer.AtKeyword:
			child = p.ParseWithFallback(func() Node { return p.Atrule(false) }, func() Node { return styleSheetConsumeRaw(p) })

		// Anything else starts a qualified rule ...
		default:
			child = p.ParseWithFallback(func() Node { return p.Rule() }, func() Node { return styleSheetConsumeRaw(p) })
		}

		children.AppendData(child)
	}

	return &StyleSheet{
		Loc:      p.Location(start, p.TokenStart),
		Children: children,
	}
}

func generateStyleSheet(g *Generator, node Node) {
	n := node.(*StyleSheet)
	g.Children(n.Children, nil)
}
