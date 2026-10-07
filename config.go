// The shape of a syntax configuration. Upstream configs are plain objects
// (see lib/syntax/config/*.js and mix.js); Config gives them a type.

package csstree

import "bytes"

// Config is a syntax configuration: the keys upstream's createSyntax,
// createParser, createGenerator, createWalker and Lexer read.
type Config struct {
	// Contexts maps a parse context (ParseOptions.Context) to the
	// function parsing the root node (upstream parseContext). Upstream
	// also allows the name of a node type as a string; in Go write a
	// function calling the node's Parser method.
	Contexts map[ParseContext]ContextParser
	// Features maps a condition kind ("supports", "container") to the
	// parsers of its functional features (e.g. supports selector()).
	Features map[string]map[string]FeatureParser
	// Scope maps a scope name ("AtrulePrelude", "Selector", "Value") to its
	// recognizer.
	Scope map[string]*Recognizer
	// Atrule maps a lowercase at-rule name to its prelude and block
	// parsers (upstream atrule[name].parse).
	Atrule map[string]AtruleParser
	// Pseudo maps a lowercase pseudo-class or pseudo-element name to the
	// parser of its arguments (upstream pseudo[name].parse).
	Pseudo map[string]PseudoParser
	// Node maps a node type name to its definition.
	Node map[string]NodeConfig

	// Lexer keys, carried for the lexer.

	// Generic is upstream generic; nil is upstream undefined (absent),
	// which Syntax.Fork does not mix into the base config.
	Generic         *bool
	CSSWideKeywords []string
	Units           map[string][]string
	Types           map[string]string
	Atrules         map[string]AtruleDefinition
	Properties      map[string]string
}

// ContextParser parses the root node of a parse context.
type ContextParser func(p *Parser, options ParseOptions) Node

// FeatureParser parses the argument of a functional feature of a
// condition, e.g. the selector of supports selector().
type FeatureParser func(p *Parser) Node

// PseudoParser parses the arguments of a functional pseudo-class or
// pseudo-element into the children of its node.
type PseudoParser func(p *Parser) *List[Node]

// AtruleParser holds the parsers of an at-rule (upstream
// atrule[name].parse = { prelude, block }). A nil field is upstream null.
type AtruleParser struct {
	// Prelude parses the at-rule prelude into the children of an
	// AtrulePrelude node.
	Prelude func(p *Parser) *List[Node]
	// Block parses the at-rule block; isDeclaration reports that the
	// at-rule is nested in a declaration block.
	Block func(p *Parser, isDeclaration bool) *Block
}

// AtruleDefinition is the syntax of an at-rule for the lexer: its prelude
// ("" is upstream null) and descriptors (nil is upstream null). A
// descriptor syntax of "" is upstream null.
type AtruleDefinition struct {
	Prelude     string
	Descriptors map[string]string
}

// MarshalJSON encodes d as upstream's {prelude, descriptors}, with null
// for an empty Prelude and a nil Descriptors.
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
