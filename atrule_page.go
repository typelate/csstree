// Ported from lib/syntax/atrule/page.js.

package csstree

func pageAtrule() AtruleParser {
	return AtruleParser{
		Prelude: func(p *Parser) *List[Node] {
			return new(List[Node]).AppendData(
				p.SelectorList(),
			)
		},
		Block: func(p *Parser, _ bool) *Block {
			return p.descriptorBlock()
		},
	}
}
