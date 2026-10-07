// Ported from lib/syntax/atrule/media.js.

package csstree

func mediaAtrule() AtruleParser {
	return AtruleParser{
		Prelude: func(p *Parser) *List[Node] {
			return new(List[Node]).AppendData(
				p.MediaQueryList(),
			)
		},
		Block: func(p *Parser, nested bool) *Block {
			return p.Block(nested)
		},
	}
}
