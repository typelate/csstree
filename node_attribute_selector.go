// Ported from lib/syntax/node/AttributeSelector.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

const (
	dollarSign       = 0x0024 // U+0024 DOLLAR SIGN ($)
	equalsSign       = 0x003D // U+003D EQUALS SIGN (=)
	circumflexAccent = 0x005E // U+005E (^)
)

func getAttributeName(p *Parser) *Identifier {
	if p.EOF {
		p.Fail("Unexpected end of input")
	}

	start := p.TokenStart
	expectIdent := false

	if p.IsDelim(asterisk, 0) {
		expectIdent = true
		p.Next()
	} else if !p.IsDelim(verticalLine, 0) {
		p.Eat(tokenizer.Ident)
	}

	if p.IsDelim(verticalLine, 0) {
		if p.CharCodeAt(p.TokenStart+1) != equalsSign {
			p.Next()
			p.Eat(tokenizer.Ident)
		} else if expectIdent {
			p.FailAt("Identifier is expected", p.TokenEnd)
		}
	} else if expectIdent {
		p.Fail("Vertical line is expected")
	}

	return &Identifier{
		Loc:  p.Location(start, p.TokenStart),
		Name: p.SubstrToCursor(start),
	}
}

func getOperator(p *Parser) string {
	start := p.TokenStart
	code := p.CharCodeAt(start)

	if code != equalsSign && // =
		code != tilde && // ~=
		code != circumflexAccent && // ^=
		code != dollarSign && // $=
		code != asterisk && // *=
		code != verticalLine { // |=
		p.Fail("Attribute selector (=, ~=, ^=, $=, *=, |=) is expected")
	}

	p.Next()

	if code != equalsSign {
		if !p.IsDelim(equalsSign, 0) {
			p.Fail("Equal sign is expected")
		}

		p.Next()
	}

	return p.SubstrToCursor(start)
}

// isAttributeModifier reports whether the current ident token is i or s
// (ASCII case-insensitive).
// https://drafts.csswg.org/selectors-4/#attribute-case
func isAttributeModifier(p *Parser) bool {
	return p.TokenEnd-p.TokenStart == 1 &&
		(p.CmpChar(p.TokenStart, 'i') || p.CmpChar(p.TokenStart, 's'))
}

// AttributeSelector is an attribute selector:
//
//	'[' <wq-name> ']'
//	'[' <wq-name> <attr-matcher> [ <string-token> | <ident-token> ] <attr-modifier>? ']'
//
//csstree:node
type AttributeSelector struct {
	Loc  *Location
	Name *Identifier `css:"name,Identifier"`
	// Matcher is the operator ("=", "~=", ...); "" is upstream null.
	Matcher string `css:"matcher,string|null"`
	// Value is a *String, an *Identifier or nil.
	Value Node `css:"value,String|Identifier|null"`
	// Flags is the modifier; "" is upstream null.
	Flags string `css:"flags,string|null"`
}

// AttributeSelector parses an attribute selector.
func (p *Parser) AttributeSelector() *AttributeSelector {
	start := p.TokenStart
	matcher := ""
	var value Node
	flags := ""

	p.Eat(tokenizer.LeftSquareBracket)
	p.SkipSC()

	name := getAttributeName(p)
	p.SkipSC()

	if p.TokenType != tokenizer.RightSquareBracket {
		// a modifier follows a matcher and a value: [name i] is not an
		// attribute selector
		matcher = getOperator(p)

		p.SkipSC()

		if p.TokenType == tokenizer.String {
			value = p.StringNode()
		} else {
			value = p.Identifier()
		}

		p.SkipSC()

		// attribute modifier: i | s
		if p.TokenType == tokenizer.Ident {
			if !isAttributeModifier(p) {
				p.Fail("Attribute selector modifier (i or s) is expected")
			}
			flags = p.Consume(tokenizer.Ident)

			p.SkipSC()
		}
	}

	p.Eat(tokenizer.RightSquareBracket)

	return &AttributeSelector{
		Loc:     p.Location(start, p.TokenStart),
		Name:    name,
		Matcher: matcher,
		Value:   value,
		Flags:   flags,
	}
}

func generateAttributeSelector(g *Generator, node Node) {
	n := node.(*AttributeSelector)
	g.Token(tokenizer.Delim, "[")
	g.Node(nodeOrNil(n.Name))

	if n.Matcher != "" {
		g.Tokenize(n.Matcher)
		g.Node(n.Value)
	}

	if n.Flags != "" {
		g.Token(tokenizer.Ident, n.Flags)
	}

	g.Token(tokenizer.Delim, "]")
}
