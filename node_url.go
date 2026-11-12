// Ported from lib/syntax/node/Url.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// URL is a url, <url-token> | <function-token> <string> ). Value is
// decoded. Its type name is "Url", as upstream names it.
//
//csstree:node name=Url
type URL struct {
	Loc   *Location
	Value string `css:"value,string"`
}

// URL parses <url-token> | <function-token> <string> ).
func (p *Parser) URL() *URL {
	start := p.TokenStart
	var value string

	switch p.TokenType {
	case tokenizer.URL:
		value = DecodeURL(p.Consume(tokenizer.URL))

	case tokenizer.Function:
		if !tokenizer.IsURLName(p.Source, p.TokenStart, p.TokenEnd-1) {
			p.Fail("Function name must be `url`")
		}

		p.Eat(tokenizer.Function)
		p.SkipSC()
		value = DecodeString(p.Consume(tokenizer.String))
		p.SkipSC()
		if !p.EOF {
			p.Eat(tokenizer.RightParenthesis)
		}

	default:
		p.Fail("Url or Function is expected")
	}

	return &URL{
		Loc:   p.Location(start, p.TokenStart),
		Value: value,
	}
}

func generateURL(g *Generator, node Node) {
	n := node.(*URL)
	g.Token(tokenizer.URL, EncodeURL(n.Value))
}
