// Ported from lib/syntax/node/Feature.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// Feature is a media or container feature: ( <name> [: <value>]? ).
//
//csstree:node
type Feature struct {
	Loc  *Location
	Kind string `css:"kind,string"`
	Name string `css:"name,string"`
	// Value is an *Identifier, a *Number, a *Dimension, a *Ratio, a
	// *Function or nil.
	Value Node `css:"value,Identifier|Number|Dimension|Ratio|Function|null"`
}

// Feature parses a feature of a condition of kind.
func (p *Parser) Feature(kind string) *Feature {
	start := p.TokenStart
	var value Node

	p.Eat(tokenizer.LeftParenthesis)
	p.SkipSC()

	name := p.Consume(tokenizer.Ident)
	p.SkipSC()

	if p.TokenType != tokenizer.RightParenthesis {
		p.Eat(tokenizer.Colon)
		p.SkipSC()

		switch p.TokenType {
		case tokenizer.Number:
			if p.LookupNonWSType(1) == tokenizer.Delim {
				value = p.Ratio()
			} else {
				value = p.Number()
			}

		case tokenizer.Dimension:
			value = p.Dimension()

		case tokenizer.Ident:
			value = p.Identifier()

		case tokenizer.Function:
			value = p.ParseWithFallback(
				func() Node {
					res := p.Function(p.ReadSequence, p.ScopeRecognizer("Value"))

					p.SkipSC()

					if p.IsDelim(solidus, 0) {
						p.Unexpected()
					}

					return res
				},
				func() Node {
					return p.Ratio()
				},
			)

		default:
			p.Fail("Number, dimension, ratio or identifier is expected")
		}

		p.SkipSC()
	}

	if !p.EOF {
		p.Eat(tokenizer.RightParenthesis)
	}

	return &Feature{
		Loc:   p.Location(start, p.TokenStart),
		Kind:  kind,
		Name:  name,
		Value: value,
	}
}

func generateFeature(g *Generator, node Node) {
	n := node.(*Feature)
	g.Token(tokenizer.LeftParenthesis, "(")
	g.Token(tokenizer.Ident, n.Name)

	if n.Value != nil {
		g.Token(tokenizer.Colon, ":")
		g.Node(n.Value)
	}

	g.Token(tokenizer.RightParenthesis, ")")
}
