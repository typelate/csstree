// Ported from lib/syntax/node/Scope.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Scope is the prelude of @scope: [(<scope-start>)]? [to (<scope-end>)]?
//
//csstree:node
type Scope struct {
	Loc *Location
	// Root and Limit are a *SelectorList, a *Raw or nil.
	Root  Node `css:"root,SelectorList|Raw|null"`
	Limit Node `css:"limit,SelectorList|Raw|null"`
}

// Scope parses the prelude of @scope.
func (p *Parser) Scope() *Scope {
	var root Node
	var limit Node

	p.SkipSC()

	startOffset := p.TokenStart
	if p.TokenType == tokenizer.LeftParenthesis {
		p.Next()
		p.SkipSC()
		root = p.ParseWithFallback(
			func() Node { return p.SelectorList() },
			func() Node { return p.Raw(nil, true) },
		)
		p.SkipSC()
		p.Eat(tokenizer.RightParenthesis)
	}

	if p.LookupNonWSType(0) == tokenizer.Ident {
		p.SkipSC()
		p.EatIdent("to")
		p.SkipSC()
		p.Eat(tokenizer.LeftParenthesis)
		p.SkipSC()
		limit = p.ParseWithFallback(
			func() Node { return p.SelectorList() },
			func() Node { return p.Raw(nil, true) },
		)
		p.SkipSC()
		p.Eat(tokenizer.RightParenthesis)
	}

	return &Scope{
		Loc:   p.Location(startOffset, p.TokenStart),
		Root:  root,
		Limit: limit,
	}
}

func generateScope(g *Generator, node Node) {
	n := node.(*Scope)
	if n.Root != nil {
		g.Token(tokenizer.LeftParenthesis, "(")
		g.Node(n.Root)
		g.Token(tokenizer.RightParenthesis, ")")
	}

	if n.Limit != nil {
		g.Token(tokenizer.Ident, "to")
		g.Token(tokenizer.LeftParenthesis, "(")
		g.Node(n.Limit)
		g.Token(tokenizer.RightParenthesis, ")")
	}
}
