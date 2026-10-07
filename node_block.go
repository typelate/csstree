// Ported from lib/syntax/node/Block.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func blockConsumeRaw(p *Parser) Node {
	return p.Raw(nil, true)
}

func blockConsumeRule(p *Parser) Node {
	return p.ParseWithFallback(func() Node { return p.Rule() }, func() Node { return blockConsumeRaw(p) })
}

// isNestedRuleAhead reports whether the tokens at the cursor of a style
// block are a nested style rule. CSS Syntax 3 "consume a block's contents"
// parses a declaration first and a qualified rule when that fails; a
// qualified rule in a block needs a {}-block before the next ;.
// Everything else is a declaration, so that what is neither falls back to
// a Raw declaration as upstream does.
// https://drafts.csswg.org/css-syntax-3/#consume-block-contents
// https://drafts.csswg.org/css-nesting-1/#syntax
func isNestedRuleAhead(p *Parser) bool {
	if p.IsDelim(ampersand, 0) {
		return true
	}
	if isDeclarationAhead(p) {
		return false
	}
	end := p.scanUntilBalanced(p.TokenIndex, tokenizer.Semicolon, tokenizer.LeftCurlyBracket)
	return p.GetTokenType(end) == tokenizer.LeftCurlyBracket
}

func blockConsumeRawDeclaration(p *Parser) Node {
	return p.Raw(p.ConsumeUntilSemicolonIncluded, true)
}

func blockConsumeDeclaration(p *Parser) Node {
	if p.TokenType == tokenizer.Semicolon {
		return blockConsumeRawDeclaration(p)
	}

	node := p.ParseWithFallback(func() Node { return p.Declaration() }, func() Node { return blockConsumeRawDeclaration(p) })

	if p.TokenType == tokenizer.Semicolon {
		p.Next()
	}

	return node
}

// Block is a {}-block of rules or declarations.
//
//csstree:node walkContext=block
type Block struct {
	Loc      *Location
	Children *List[Node] `css:"children,[Atrule|Rule|Declaration]"`
}

// Block parses a {}-block; isStyleBlock makes it a block of declarations
// and nested rules instead of a block of rules.
func (p *Parser) Block(isStyleBlock bool) *Block {
	return p.block(isStyleBlock, isStyleBlock)
}

// descriptorBlock parses a {}-block of declarations (descriptors) of an
// at-rule such as @font-face or @page, where a nested style rule is not
// valid: only a block item that starts with & is read as a rule, as
// upstream does, and what looks like a rule is a declaration.
func (p *Parser) descriptorBlock() *Block {
	return p.block(true, false)
}

// block parses a {}-block; isStyleBlock makes it a block of declarations,
// and nestedRules reads nested style rules in it.
func (p *Parser) block(isStyleBlock, nestedRules bool) *Block {
	consumer := blockConsumeRule
	if isStyleBlock {
		consumer = blockConsumeDeclaration
	}
	start := p.TokenStart
	children := new(List[Node])

	p.Eat(tokenizer.LeftCurlyBracket)

scan:
	for !p.EOF {
		switch p.TokenType {
		case tokenizer.RightCurlyBracket:
			break scan

		case tokenizer.WhiteSpace, tokenizer.Comment:
			p.Next()

		case tokenizer.AtKeyword:
			children.AppendData(p.ParseWithFallback(func() Node { return p.Atrule(isStyleBlock) }, func() Node { return blockConsumeRaw(p) }))

		default:
			if isStyleBlock && (p.IsDelim(ampersand, 0) || nestedRules && isNestedRuleAhead(p)) {
				children.AppendData(blockConsumeRule(p))
			} else {
				children.AppendData(consumer(p))
			}
		}
	}

	if !p.EOF {
		p.Eat(tokenizer.RightCurlyBracket)
	}

	return &Block{
		Loc:      p.Location(start, p.TokenStart),
		Children: children,
	}
}

func generateBlock(g *Generator, node Node) {
	n := node.(*Block)
	g.Token(tokenizer.LeftCurlyBracket, "{")
	g.Children(n.Children, func(prev Node) {
		if _, ok := prev.(*Declaration); ok {
			g.Token(tokenizer.Semicolon, ";")
		}
	})
	g.Token(tokenizer.RightCurlyBracket, "}")
}
