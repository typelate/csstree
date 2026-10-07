// Ported from lib/syntax/node/Rule.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func ruleConsumeRaw(p *Parser) Node {
	return p.Raw(p.ConsumeUntilLeftCurlyBracket, true)
}

func consumePrelude(p *Parser) Node {
	prelude := p.SelectorList()

	// upstream also lets a Raw prelude through, which SelectorList never
	// returns
	if !p.EOF && p.TokenType != tokenizer.LeftCurlyBracket {
		p.Unexpected()
	}

	return prelude
}

// Rule is a qualified rule: a prelude and a block.
//
//csstree:node walkContext=rule
type Rule struct {
	Loc *Location
	// Prelude is a *SelectorList or a *Raw.
	Prelude Node   `css:"prelude,SelectorList|Raw"`
	Block   *Block `css:"block,Block"`
}

// Rule parses a qualified rule.
func (p *Parser) Rule() *Rule {
	startOffset := p.TokenStart
	var prelude Node

	if p.ParseRulePrelude {
		prelude = p.ParseWithFallback(func() Node { return consumePrelude(p) }, func() Node { return ruleConsumeRaw(p) })
	} else {
		prelude = ruleConsumeRaw(p)
	}

	block := p.Block(true)

	return &Rule{
		Loc:     p.Location(startOffset, p.TokenStart),
		Prelude: prelude,
		Block:   block,
	}
}

func generateRule(g *Generator, node Node) {
	n := node.(*Rule)
	g.Node(n.Prelude)
	g.Node(nodeOrNil(n.Block))
}
