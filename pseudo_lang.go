// Ported from lib/syntax/pseudo/lang.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func parseLanguageRangeList(p *Parser) *List[Node] {
	children := new(List[Node])

	p.SkipSC()

loop:
	for !p.EOF {
		switch p.TokenType {
		case tokenizer.Ident:
			children.AppendData(p.Identifier())

		case tokenizer.String:
			children.AppendData(p.StringNode())

		case tokenizer.Comma:
			children.AppendData(p.Operator())

		case tokenizer.RightParenthesis:
			break loop

		default:
			p.Fail("Identifier, string or comma is expected")
		}

		p.SkipSC()
	}

	return children
}
