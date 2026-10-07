// Ported from lib/syntax/atrule/scope.js.

package csstree

func scopeAtrule() AtruleParser {
	return AtruleParser{
		Prelude: func(p *Parser) *List[Node] {
			return new(List[Node]).AppendData(
				p.Scope(),
			)
		},
		Block: func(p *Parser, nested bool) *Block {
			return p.Block(nested)
		},
	}
}
