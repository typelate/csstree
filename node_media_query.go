// Ported from lib/syntax/node/MediaQuery.js.

package csstree

import (
	"github.com/typelate/csstree/tokenizer"
)

// MediaQuery is a media query.
//
//csstree:node
type MediaQuery struct {
	Loc *Location
	// Modifier is "not", "only" or "" for none.
	Modifier string `css:"modifier,string|null"`
	// MediaType is the media type, or "" for none.
	MediaType string     `css:"mediaType,string|null"`
	Condition *Condition `css:"condition,Condition|null"`
}

// MediaQuery parses a media query.
func (p *Parser) MediaQuery() *MediaQuery {
	start := p.TokenStart
	modifier := ""
	mediaType := ""
	var condition *Condition

	p.SkipSC()

	if p.TokenType == tokenizer.Ident && p.LookupTypeNonSC(1) != tokenizer.LeftParenthesis {
		// [ not | only ]? <media-type>
		ident := p.Consume(tokenizer.Ident)
		identLowerCase := toLowerCase(ident)

		if identLowerCase == "not" || identLowerCase == "only" {
			p.SkipSC()
			modifier = identLowerCase
			mediaType = p.Consume(tokenizer.Ident)
		} else {
			mediaType = ident
		}

		switch p.LookupTypeNonSC(0) {
		case tokenizer.Ident:
			// and <media-condition-without-or>
			p.SkipSC()
			p.EatIdent("and")
			condition = p.condition("media", false)

		case tokenizer.LeftCurlyBracket, tokenizer.Semicolon, tokenizer.Comma, tokenizer.EOF:

		default:
			p.Fail("Identifier or parenthesis is expected")
		}
	} else {
		switch p.TokenType {
		case tokenizer.Ident, tokenizer.LeftParenthesis, tokenizer.Function:
			// <media-condition>
			condition = p.Condition("media")

		case tokenizer.LeftCurlyBracket, tokenizer.Semicolon, tokenizer.EOF:

		default:
			p.Fail("Identifier or parenthesis is expected")
		}
	}

	return &MediaQuery{
		Loc:       p.Location(start, p.TokenStart),
		Modifier:  modifier,
		MediaType: mediaType,
		Condition: condition,
	}
}

func generateMediaQuery(g *Generator, node Node) {
	n := node.(*MediaQuery)
	if n.MediaType != "" {
		if n.Modifier != "" {
			g.Token(tokenizer.Ident, n.Modifier)
		}

		g.Token(tokenizer.Ident, n.MediaType)

		if n.Condition != nil {
			g.Token(tokenizer.Ident, "and")
			g.Node(n.Condition)
		}
	} else if n.Condition != nil {
		g.Node(n.Condition)
	}
}
