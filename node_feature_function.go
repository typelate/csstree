// Ported from lib/syntax/node/FeatureFunction.js.

package csstree

import (
	"github.com/typelate/csstree/tokenizer"
)

// FeatureFunction is a functional feature of a condition, e.g.
// selector() in @supports or style() in @container.
//
//csstree:node
type FeatureFunction struct {
	Loc     *Location
	Kind    string `css:"kind,string"`
	Feature string `css:"feature,string"`
	// Value is a *Declaration, a *Selector (or a *Raw when the value does
	// not parse).
	Value Node `css:"value,Declaration|Selector"`
}

func getFeatureParser(p *Parser, kind, name string) FeatureParser {
	featuresOfKind := p.config.Features[kind]
	parser := featuresOfKind[name]

	if parser == nil {
		p.Fail("Unknown feature " + name + "()")
	}

	return parser
}

// FeatureFunction parses a functional feature of a condition of kind (""
// means "unknown").
func (p *Parser) FeatureFunction(kind string) *FeatureFunction {
	if kind == "" {
		kind = "unknown"
	}
	start := p.TokenStart
	functionName := p.ConsumeFunctionName()
	valueParser := getFeatureParser(p, kind, toLowerCase(functionName))

	p.SkipSC()

	value := p.ParseWithFallback(
		func() Node {
			startValueToken := p.TokenIndex
			value := valueParser(p)

			if !p.EOF &&
				!p.IsBalanceEdge(startValueToken) {
				p.Unexpected()
			}

			return value
		},
		func() Node { return p.Raw(nil, false) },
	)

	if !p.EOF {
		p.Eat(tokenizer.RightParenthesis)
	}

	return &FeatureFunction{
		Loc:     p.Location(start, p.TokenStart),
		Kind:    kind,
		Feature: functionName,
		Value:   value,
	}
}

func generateFeatureFunction(g *Generator, node Node) {
	n := node.(*FeatureFunction)
	g.Token(tokenizer.Function, n.Feature+"(")
	g.Node(n.Value)
	g.Token(tokenizer.RightParenthesis, ")")
}
