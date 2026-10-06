// Ported from lib/syntax/node/Rule.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func ruleConsumeRaw(p *Parser) Node {
	return p.Raw(p.ConsumeUntilLeftCurlyBracket, true)
}

func consumePrelude(p *Parser) Node {
	prelude := p.SelectorList()

	if prelude.Type() != "Raw" &&
		!p.EOF &&
		p.TokenType != tokenizer.LeftCurlyBracket {
		p.Error("", -1)
	}

	return prelude
}

// Rule is a qualified rule: a prelude and a block.
type Rule struct {
	Loc *Location
	// Prelude is a *SelectorList or a *Raw.
	Prelude Node
	Block   *Block
}

func (n *Rule) Type() string        { return "Rule" }
func (n *Rule) Location() *Location { return n.Loc }

func (n *Rule) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"prelude", n.Prelude},
		jsonField{"block", n.Block},
	)
}

func ruleNodeType() NodeConfig {
	return NodeConfig{
		Name:        "Rule",
		WalkContext: "rule",
		Structure: []StructureField{
			{Name: "prelude", Types: []FieldType{nodeField("SelectorList"), nodeField("Raw")}},
			{Name: "block", Types: []FieldType{nodeField("Block")}},
		},
		Generate: generateRule,
	}
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
		Loc:     p.GetLocation(startOffset, p.TokenStart),
		Prelude: prelude,
		Block:   block,
	}
}

func generateRule(g *Generator, node Node) {
	n := node.(*Rule)
	g.Node(n.Prelude)
	g.Node(n.Block)
}
