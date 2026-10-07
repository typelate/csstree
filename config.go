// The shape of a syntax configuration. Upstream configs are plain objects
// (see lib/syntax/config/*.js and mix.js); Config gives them a type.

package csstree

import "bytes"

// Config is a syntax configuration: what the parser, generator, walker and
// lexer of a Syntax are built from. NewSyntax builds a syntax from a
// Config, and Fork extends one with a Config (see SyntaxExtension).
type Config struct {
	// Contexts maps a parse context (ParseOptions.Context) to the
	// function parsing the root node. Such a function usually calls a
	// Parser method, such as p.Value().
	Contexts map[ParseContext]ContextParser
	// Features maps a condition kind ("supports", "container") to the
	// parsers of its functional features (e.g. supports selector()).
	Features map[string]map[string]FeatureParser
	// Scope maps a scope name ("AtrulePrelude", "Selector", "Value") to its
	// recognizer.
	Scope map[string]*Recognizer
	// Atrule maps a lowercase at-rule name to its prelude and block
	// parsers.
	Atrule map[string]AtruleParser
	// Pseudo maps a lowercase pseudo-class or pseudo-element name to the
	// parser of its arguments.
	Pseudo map[string]PseudoParser
	// Node maps a node type name to its definition. Every node type the
	// parser can return needs one.
	Node map[string]NodeConfig

	// Lexer keys, carried for the lexer.

	// Generic reports whether the lexer has the generic types, such as
	// <length> and <custom-ident>, that have no definition syntax. nil leaves
	// the setting of the config being extended, and means false for
	// NewSyntax.
	Generic *bool
	// CSSWideKeywords are the keywords every property accepts, such as
	// "inherit". A Config passed to Fork adds to them.
	CSSWideKeywords []string
	// Units maps a unit group, such as "length", to its units.
	Units map[string][]string
	// Types maps a type name, such as "color", to its definition syntax.
	// In a Config passed to Fork, a syntax starting with "|" adds
	// alternatives to the existing one.
	Types map[string]string
	// Atrules maps an at-rule name, without "@", to its prelude and
	// descriptor syntaxes.
	Atrules map[string]AtruleDefinition
	// Properties maps a property name to its definition syntax. In a
	// Config passed to Fork, a syntax starting with "|" adds alternatives
	// to the existing one.
	Properties map[string]string
}

// ContextParser parses the root node of a parse context.
type ContextParser func(p *Parser, options ParseOptions) Node

// FeatureParser parses the argument of a functional feature of a
// condition, e.g. the selector of supports selector().
type FeatureParser func(p *Parser) Node

// PseudoParser parses the arguments of a functional pseudo-class or
// pseudo-element into the children of its node.
type PseudoParser func(p *Parser) *List[Node]

// AtruleParser holds the parsers of an at-rule. A nil Prelude parses the
// prelude with the AtrulePrelude scope, and a nil Block parses the block
// as Parser.Atrule does for an at-rule without a parser: as a block of
// declarations when it holds no nested block or at-rule, else as a block
// of rules.
type AtruleParser struct {
	// Prelude parses the at-rule prelude into the children of an
	// AtrulePrelude node.
	Prelude func(p *Parser) *List[Node]
	// Block parses the at-rule block; isDeclaration reports that the
	// at-rule is nested in a declaration block.
	Block func(p *Parser, isDeclaration bool) *Block
}

// AtruleDefinition is the syntax of an at-rule for the lexer.
type AtruleDefinition struct {
	// Prelude is the definition syntax of the prelude, or "" when the
	// at-rule has no prelude.
	Prelude string
	// Descriptors maps a descriptor name to its definition syntax; a ""
	// syntax, or a nil map, means none.
	Descriptors map[string]string
}

// MarshalJSON encodes d as {"prelude": ..., "descriptors": ...}, with null
// for an empty Prelude and a nil Descriptors, as upstream's dump does.
func (d AtruleDefinition) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`{"prelude":`)
	if err := writeJSON(&buf, nullableString(d.Prelude)); err != nil {
		return nil, err
	}
	buf.WriteString(`,"descriptors":`)
	var descriptors any
	if d.Descriptors != nil {
		descriptors = d.Descriptors
	}
	if err := writeJSON(&buf, descriptors); err != nil {
		return nil, err
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
