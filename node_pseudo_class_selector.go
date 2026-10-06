// Ported from lib/syntax/node/PseudoClassSelector.js.

package csstree

import (
	"strings"

	"github.com/typelate/csstree/tokenizer"
)

// PseudoClassSelector is : [ <ident> | <function-token> <any-value>? ) ].
type PseudoClassSelector struct {
	Loc  *Location
	Name string
	// Children holds the arguments of a functional pseudo-class; nil for a
	// pseudo-class written without parentheses.
	Children *List[Node]
}

func (n *PseudoClassSelector) Type() string        { return "PseudoClassSelector" }
func (n *PseudoClassSelector) Location() *Location { return n.Loc }

func (n *PseudoClassSelector) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"name", n.Name},
		jsonField{"children", n.Children},
	)
}

func pseudoClassSelectorNodeType() NodeConfig {
	return NodeConfig{
		Name:        "PseudoClassSelector",
		WalkContext: "function",
		Structure: []StructureField{
			{Name: "name", Types: []FieldType{stringField()}},
			{Name: "children", Types: []FieldType{listField("Raw"), nullField()}},
		},
		Generate: generatePseudoClassSelector,
	}
}

// PseudoClassSelector parses : [ <ident> | <function-token> <any-value>? ) ]
func (p *Parser) PseudoClassSelector() *PseudoClassSelector {
	start := p.TokenStart
	var children *List[Node]
	var name string

	p.Eat(tokenizer.Colon)

	if p.TokenType == tokenizer.Function {
		name = p.ConsumeFunctionName()
		nameLowerCase := strings.ToLower(name)

		if p.LookupNonWSType(0) == tokenizer.RightParenthesis {
			children = p.CreateList()
		} else if pseudo, ok := p.Config.Pseudo[nameLowerCase]; ok {
			p.SkipSC()
			children = pseudo(p)
			p.SkipSC()
		} else {
			children = p.CreateList()
			children.Push(
				p.Raw(nil, false),
			)
		}

		p.Eat(tokenizer.RightParenthesis)
	} else {
		name = p.Consume(tokenizer.Ident)
	}

	return &PseudoClassSelector{
		Loc:      p.GetLocation(start, p.TokenStart),
		Name:     name,
		Children: children,
	}
}

func generatePseudoClassSelector(g *Generator, node Node) {
	n := node.(*PseudoClassSelector)
	g.Token(tokenizer.Colon, ":")

	if n.Children == nil {
		g.Token(tokenizer.Ident, n.Name)
	} else {
		g.Token(tokenizer.Function, n.Name+"(")
		g.Children(n.Children, nil)
		g.Token(tokenizer.RightParenthesis, ")")
	}
}
