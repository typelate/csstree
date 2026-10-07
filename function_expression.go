// Ported from lib/syntax/function/expression.js.

package csstree

// legacy IE function
// expression( <any-value> )
func expressionFunction(p *Parser, _ *Recognizer) *List[Node] {
	return new(List[Node]).AppendData(
		p.Raw(nil, false),
	)
}
