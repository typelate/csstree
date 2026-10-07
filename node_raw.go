// Ported from lib/syntax/node/Raw.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func getOffsetExcludeWS(p *Parser) int {
	if p.TokenIndex > 0 {
		if p.LookupType(-1) == tokenizer.WhiteSpace {
			if p.TokenIndex > 1 {
				return p.GetTokenStart(p.TokenIndex - 1)
			}
			return p.GetTokenStart(0) // upstream this.firstCharOffset
		}
	}

	return p.TokenStart
}

// Raw is source text kept as is.
//
//csstree:node
type Raw struct {
	Loc   *Location
	Value string `css:"value,string"`
}

// Raw consumes tokens up to the end of the enclosing block, or until
// consumeUntil stops (see the Parser.ConsumeUntil* methods); a nil
// consumeUntil never stops early. excludeWhiteSpace leaves trailing
// whitespace out of the value.
func (p *Parser) Raw(consumeUntil tokenizer.StopFunc, excludeWhiteSpace bool) *Raw {
	startOffset := p.GetTokenStart(p.TokenIndex)
	var endOffset int

	p.SkipUntilBalanced(p.TokenIndex, consumeUntil)

	if excludeWhiteSpace && p.TokenStart > startOffset {
		endOffset = getOffsetExcludeWS(p)
	} else {
		endOffset = p.TokenStart
	}

	return &Raw{
		Loc:   p.Location(startOffset, endOffset),
		Value: p.Substring(startOffset, endOffset),
	}
}

func generateRaw(g *Generator, node Node) {
	n := node.(*Raw)
	g.Tokenize(n.Value)
}
