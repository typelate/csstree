// Ported from lib/syntax/node/Declaration.js.

package csstree

import "github.com/typelate/csstree/tokenizer"

func consumeValueRaw(p *Parser) Node {
	return p.Raw(p.ConsumeUntilExclamationMarkOrSemicolon, true)
}

func consumeCustomPropertyRaw(p *Parser) Node {
	return p.Raw(p.ConsumeUntilExclamationMarkOrSemicolon, false)
}

func consumeValue(p *Parser) Node {
	startValueToken := p.TokenIndex
	value := p.Value()

	if value.Type() != "Raw" &&
		!p.EOF &&
		p.TokenType != tokenizer.Semicolon &&
		!p.IsDelim(exclamationMark, 0) &&
		!p.IsBalanceEdge(startValueToken) {
		p.Error("", -1)
	}

	return value
}

// Declaration is a property declaration.
type Declaration struct {
	Loc *Location
	// Important is "" for no !important (upstream false), "important" for
	// !important (upstream true), or the text after ! when it differs from
	// "important", e.g. "IMPORTANT" or "ie" (upstream keeps that string).
	Important string
	Property  string
	// Value is a *Value or a *Raw.
	Value Node
}

func (n *Declaration) Type() string        { return "Declaration" }
func (n *Declaration) Location() *Location { return n.Loc }

func (n *Declaration) MarshalJSON() ([]byte, error) {
	var important any
	switch n.Important {
	case "":
		important = false
	case "important":
		important = true
	default:
		important = n.Important
	}
	return marshalNode(n,
		jsonField{"important", important},
		jsonField{"property", n.Property},
		jsonField{"value", n.Value},
	)
}

func declarationNodeType() NodeConfig {
	return NodeConfig{
		Name:        "Declaration",
		WalkContext: "declaration",
		Structure: []StructureField{
			{Name: "important", Types: []FieldType{booleanField(), stringField()}},
			{Name: "property", Types: []FieldType{stringField()}},
			{Name: "value", Types: []FieldType{nodeField("Value"), nodeField("Raw")}},
		},
		Generate:        generateDeclaration,
		Walk:            walkDeclaration,
		Clone:           cloneDeclaration,
		FromPlainObject: fromPlainObjectDeclaration,
	}
}

func walkDeclaration(node Node, _ bool, walkNode func(Node) bool, _ func(*List[Node]) bool) bool {
	n := node.(*Declaration)
	return n.Value != nil && walkNode(n.Value)
}

func cloneDeclaration(node Node, clone func(Node) Node) Node {
	c := *node.(*Declaration)
	c.Loc = cloneLocation(c.Loc)
	c.Value = clone(c.Value)
	return &c
}

func fromPlainObjectDeclaration(r *PlainObjectReader, object map[string]any) Node {
	return &Declaration{
		Loc:       r.Location(object),
		Important: plainObjectImportant(r, object),
		Property:  r.String(object, "property"),
		Value:     r.Node(object, "value"),
	}
}

// plainObjectImportant reads Declaration.important: false, true or a
// string.
func plainObjectImportant(r *PlainObjectReader, object map[string]any) string {
	switch value := object["important"].(type) {
	case nil:
		return ""
	case bool:
		if value {
			return "important"
		}
		return ""
	case string:
		return value
	default:
		r.fieldError(object, "important", "a boolean or a string")
		return ""
	}
}

// Declaration parses a declaration.
func (p *Parser) Declaration() *Declaration {
	start := p.TokenStart
	startToken := p.TokenIndex
	property := readProperty(p)
	customProperty := IsCustomProperty(property, 0)
	parseValue := p.ParseValue
	consumeRaw := consumeValueRaw
	if customProperty {
		parseValue = p.ParseCustomProperty
		consumeRaw = consumeCustomPropertyRaw
	}
	important := ""
	var value Node

	p.SkipSC()
	p.Eat(tokenizer.Colon)

	valueStart := p.TokenIndex

	if !customProperty {
		p.SkipSC()
	}

	if parseValue {
		value = p.ParseWithFallback(func() Node { return consumeValue(p) }, func() Node { return consumeRaw(p) })
	} else {
		value = consumeRaw(p)
	}

	if value, ok := value.(*Value); customProperty && ok && value.Children.IsEmpty() {
		for offset := valueStart - p.TokenIndex; offset <= 0; offset++ {
			if p.LookupType(offset) == tokenizer.WhiteSpace {
				value.Children.AppendData(&WhiteSpace{
					Loc:   nil,
					Value: " ",
				})
				break
			}
		}
	}

	if p.IsDelim(exclamationMark, 0) {
		important = getImportant(p)
		p.SkipSC()
	}

	// Do not include semicolon to range per spec
	// https://drafts.csswg.org/css-syntax/#declaration-diagram

	if !p.EOF &&
		p.TokenType != tokenizer.Semicolon &&
		!p.IsBalanceEdge(startToken) {
		p.Error("", -1)
	}

	return &Declaration{
		Loc:       p.GetLocation(start, p.TokenStart),
		Important: important,
		Property:  property,
		Value:     value,
	}
}

func generateDeclaration(g *Generator, node Node) {
	n := node.(*Declaration)
	g.Token(tokenizer.Ident, n.Property)
	g.Token(tokenizer.Colon, ":")
	g.Node(n.Value)

	if n.Important != "" {
		g.Token(tokenizer.Delim, "!")
		g.Token(tokenizer.Ident, n.Important)
	}
}

func readProperty(p *Parser) string {
	start := p.TokenStart

	// hacks
	if p.TokenType == tokenizer.Delim {
		switch p.CharCodeAt(p.TokenStart) {
		case asterisk, dollarSign, plusSign, numberSign, ampersand:
			p.Next()

		// TODO: not sure we should support this hack
		case solidus:
			p.Next()
			if p.IsDelim(solidus, 0) {
				p.Next()
			}
		}
	}

	if p.TokenType == tokenizer.Hash {
		p.Eat(tokenizer.Hash)
	} else {
		p.Eat(tokenizer.Ident)
	}

	return p.SubstrToCursor(start)
}

// ! ws* important
func getImportant(p *Parser) string {
	p.Eat(tokenizer.Delim)
	p.SkipSC()

	important := p.Consume(tokenizer.Ident)

	// store original value in case it differ from `important`
	// for better original source restoring and hacks like `!ie` support
	// (upstream returns true for "important"; Declaration.Important keeps
	// the string)
	return important
}
