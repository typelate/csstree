// Ported from lib/syntax/node/FeatureRange.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

const lessThanSign = 0x003C // U+003C LESS-THAN SIGN (<)

// FeatureRange is a range feature of a media or container condition, e.g.
// (400px < width <= 700px).
//
//csstree:node
type FeatureRange struct {
	Loc  *Location
	Kind string `css:"kind,string"`
	// Left, Middle and Right are an *Identifier, a *Number, a *Dimension, a
	// *Ratio or a *Function; Right may be nil.
	Left           Node   `css:"left,Identifier|Number|Dimension|Ratio|Function"`
	LeftComparison string `css:"leftComparison,string"`
	Middle         Node   `css:"middle,Identifier|Number|Dimension|Ratio|Function"`
	// RightComparison is "" (upstream null) when Right is nil.
	RightComparison string `css:"rightComparison,string|null"`
	Right           Node   `css:"right,Identifier|Number|Dimension|Ratio|Function|null"`
}

func readTerm(p *Parser) Node {
	p.SkipSC()

	switch p.TokenType {
	case tokenizer.Number:
		if p.IsDelim(solidus, p.LookupOffsetNonSC(1)) {
			return p.Ratio()
		} else {
			return p.Number()
		}

	case tokenizer.Dimension:
		return p.Dimension()

	case tokenizer.Ident:
		return p.Identifier()

	case tokenizer.Function:
		return p.ParseWithFallback(
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

	return nil // unreachable: Fail panics
}

func readComparison(p *Parser, expectColon bool) string {
	p.SkipSC()

	if p.IsDelim(lessThanSign, 0) ||
		p.IsDelim(greaterThanSign, 0) {
		value := p.Source[p.TokenStart : p.TokenStart+1]

		p.Next()

		if p.IsDelim(equalsSign, 0) {
			p.Next()
			return value + "="
		}

		return value
	}

	if p.IsDelim(equalsSign, 0) {
		p.Next()
		return "="
	}

	colon := ""
	if expectColon {
		colon = `":", `
	}
	p.Fail(`Expected ` + colon + `"<", ">", "=" or ")"`)

	return "" // unreachable: Fail panics
}

// checkRangeDirection reports a syntax error at the right comparison
// (just read) of a range with two comparisons unless both are < or <=, or
// both are > or >=: there is no = in a range with two comparisons.
// https://drafts.csswg.org/mediaqueries-4/#mq-range-context
func checkRangeDirection(p *Parser, leftComparison, rightComparison string) {
	offset := p.TokenStart - len(rightComparison)
	switch leftComparison[0] {
	case '<':
		if rightComparison[0] != '<' {
			p.FailAt(`Expected "<", "<=" or ")"`, offset)
		}
	case '>':
		if rightComparison[0] != '>' {
			p.FailAt(`Expected ">", ">=" or ")"`, offset)
		}
	default:
		p.FailAt(`")" is expected`, offset)
	}
}

// FeatureRange parses a range feature of a condition of kind ("" means
// "unknown").
func (p *Parser) FeatureRange(kind string) *FeatureRange {
	if kind == "" {
		kind = "unknown"
	}
	start := p.TokenStart

	p.SkipSC()
	p.Eat(tokenizer.LeftParenthesis)

	left := readTerm(p)
	_, leftIsIdentifier := left.(*Identifier)
	leftComparison := readComparison(p, leftIsIdentifier)
	middle := readTerm(p)
	rightComparison := ""
	var right Node

	if p.LookupNonWSType(0) != tokenizer.RightParenthesis {
		rightComparison = readComparison(p, false)
		checkRangeDirection(p, leftComparison, rightComparison)
		right = readTerm(p)
	}

	p.SkipSC()
	p.Eat(tokenizer.RightParenthesis)

	return &FeatureRange{
		Loc:             p.Location(start, p.TokenStart),
		Kind:            kind,
		Left:            left,
		LeftComparison:  leftComparison,
		Middle:          middle,
		RightComparison: rightComparison,
		Right:           right,
	}
}

func generateFeatureRange(g *Generator, node Node) {
	n := node.(*FeatureRange)
	g.Token(tokenizer.LeftParenthesis, "(")
	g.Node(n.Left)
	g.Tokenize(n.LeftComparison)
	g.Node(n.Middle)

	if n.Right != nil {
		g.Tokenize(n.RightComparison)
		g.Node(n.Right)
	}

	g.Token(tokenizer.RightParenthesis, ")")
}
