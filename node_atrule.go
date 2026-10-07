// Ported from lib/syntax/node/Atrule.js.

package csstree

import (
	"github.com/typelate/csstree/tokenizer"
)

func atruleConsumeRaw(p *Parser) Node {
	return p.Raw(p.ConsumeUntilLeftCurlyBracketOrSemicolon, true)
}

func isDeclarationBlockAtrule(p *Parser) bool {
	for offset := 1; ; offset++ {
		tokenType := p.LookupType(offset)
		if tokenType == tokenizer.EOF {
			break
		}

		if tokenType == tokenizer.RightCurlyBracket {
			return true
		}

		if tokenType == tokenizer.LeftCurlyBracket ||
			tokenType == tokenizer.AtKeyword {
			return false
		}
	}

	return false
}

// Atrule is an at-rule such as @media or @import.
//
//csstree:node walkContext=atrule
type Atrule struct {
	Loc  *Location
	Name string `css:"name,string"`
	// Prelude is an *AtrulePrelude, a *Raw or nil.
	Prelude Node `css:"prelude,AtrulePrelude|Raw|null"`
	// Block is nil for an at-rule ending with a semicolon.
	Block *Block `css:"block,Block|null"`
}

// Atrule parses an at-rule; isDeclaration reports that it is nested in a
// declaration block.
func (p *Parser) Atrule(isDeclaration bool) *Atrule {
	start := p.TokenStart
	var prelude Node
	var block *Block

	p.Eat(tokenizer.AtKeyword)

	name := p.SubstrToCursor(start + 1)
	nameLowerCase := toLowerCase(name)
	p.SkipSC()

	// parse prelude
	if !p.EOF &&
		p.TokenType != tokenizer.LeftCurlyBracket &&
		p.TokenType != tokenizer.Semicolon {
		if p.ParseAtrulePrelude {
			prelude = p.ParseWithFallback(func() Node { return p.AtrulePrelude(name) }, func() Node { return atruleConsumeRaw(p) })
		} else {
			prelude = atruleConsumeRaw(p)
		}

		p.SkipSC()
	}

	switch p.TokenType {
	case tokenizer.Semicolon:
		p.Next()

	case tokenizer.LeftCurlyBracket:
		if atrule, ok := p.config.Atrule[nameLowerCase]; ok &&
			atrule.Block != nil {
			block = atrule.Block(p, isDeclaration)
		} else {
			// TODO: should consume block content as Raw?
			block = p.Block(isDeclarationBlockAtrule(p))
		}
	}

	return &Atrule{
		Loc:     p.Location(start, p.TokenStart),
		Name:    name,
		Prelude: prelude,
		Block:   block,
	}
}

func generateAtrule(g *Generator, node Node) {
	n := node.(*Atrule)
	g.Token(tokenizer.AtKeyword, "@"+n.Name)

	if n.Prelude != nil {
		g.Node(n.Prelude)
	}

	if n.Block != nil {
		g.Node(n.Block)
	} else {
		g.Token(tokenizer.Semicolon, ";")
	}
}
