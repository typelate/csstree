// Ported from lib/syntax/node/GeneralEnclosed.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

// GeneralEnclosed is a condition term the parser does not know:
//
//	<function-token> <any-value> )
//	( <any-value> )
type GeneralEnclosed struct {
	Loc  *Location
	Kind string
	// Function is the function name; "" (upstream null) for ( <any-value> ).
	Function string
	Children *List[Node]
}

func (n *GeneralEnclosed) Type() string        { return "GeneralEnclosed" }
func (n *GeneralEnclosed) Location() *Location { return n.Loc }

func (n *GeneralEnclosed) MarshalJSON() ([]byte, error) {
	return marshalNode(n,
		jsonField{"kind", n.Kind},
		jsonField{"function", nullableString(n.Function)},
		jsonField{"children", n.Children},
	)
}

func generalEnclosedNodeType() NodeConfig {
	return NodeConfig{
		Name: "GeneralEnclosed",
		Structure: []StructureField{
			{Name: "kind", Types: []FieldType{stringField()}},
			{Name: "function", Types: []FieldType{stringField(), nullField()}},
			{Name: "children", Types: []FieldType{listField()}},
		},
		Generate: generateGeneralEnclosed,
	}
}

// GeneralEnclosed parses a general enclosed term of a condition of kind.
func (p *Parser) GeneralEnclosed(kind string) *GeneralEnclosed {
	start := p.TokenStart
	functionName := ""

	if p.TokenType == tokenizer.Function {
		functionName = p.ConsumeFunctionName()
	} else {
		p.Eat(tokenizer.LeftParenthesis)
	}

	var children *List[Node]
	p.ParseWithFallback(
		func() Node {
			startValueToken := p.TokenIndex
			children = p.ReadSequence(p.Config.Scope["Value"])

			if !p.EOF &&
				!p.IsBalanceEdge(startValueToken) {
				p.Error("", -1)
			}

			return nil
		},
		func() Node {
			children = p.CreateSingleNodeList(
				p.Raw(nil, false),
			)
			return children.First()
		},
	)

	if !p.EOF {
		p.Eat(tokenizer.RightParenthesis)
	}

	return &GeneralEnclosed{
		Loc:      p.GetLocation(start, p.TokenStart),
		Kind:     kind,
		Function: functionName,
		Children: children,
	}
}

func generateGeneralEnclosed(g *Generator, node Node) {
	n := node.(*GeneralEnclosed)
	if n.Function != "" {
		g.Token(tokenizer.Function, n.Function+"(")
	} else {
		g.Token(tokenizer.LeftParenthesis, "(")
	}

	g.Children(n.Children, nil)
	g.Token(tokenizer.RightParenthesis, ")")
}
