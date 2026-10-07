// Ported from lib/syntax/node/index.js and the node type definition shape
// (name, structure, walkContext, generate) shared by lib/syntax/node/*.js.
//
// The node types of the package declare their structure in css struct
// tags; nodegen writes their methods, NodeConfigs and Walk, Clone and
// FromPlainObject functions to nodes_gen.go (see internal/nodegen).

//go:generate go run ./internal/cmd/nodegen

package csstree

import (
	"bytes"
	"encoding/json"

	"github.com/typelate/csstree/tokenizer"
)

// Node is a CSS AST node: a pointer to one of the node structs of this
// package (StyleSheet, Rule, Selector, ...) or to a node type added by an
// extension.
type Node interface {
	// Type returns the name of the node type, such as "Selector". It is
	// the "type" key of the node's JSON.
	Type() string
	// Location returns the node's range in the source, or nil when the
	// parse did not record positions (see ParseOptions.Positions).
	Location() *Location
}

// Location is a node's range in the source. Its JSON is the "loc" key of a
// node.
type Location = tokenizer.Location

// FieldKind is the kind of one alternative of a node structure field.
type FieldKind int

const (
	// FieldString means the field holds a string.
	FieldString FieldKind = iota + 1
	// FieldBoolean means the field holds a boolean.
	FieldBoolean
	// FieldNull means the field may be null (a nil node or list, or an
	// empty string).
	FieldNull
	// FieldNode means the field holds a node of type FieldType.Node.
	FieldNode
	// FieldList means the field holds a list of nodes whose types are in
	// FieldType.List (any type when it is empty).
	FieldList
)

// FieldType is one alternative of a node structure field.
type FieldType struct {
	// Kind is what the field holds.
	Kind FieldKind
	// Node is the node type for FieldNode.
	Node string
	// List holds the allowed item node types for FieldList; empty means any.
	List []string
}

// StructureField describes one field of a node type: its JSON key and the
// kinds of value it may hold. The lexer's structure check and the walk
// order follow the order of the fields in NodeConfig.Structure.
type StructureField struct {
	// Name is the field's JSON key, such as "children".
	Name string
	// Types are the kinds of value the field may hold.
	Types []FieldType
}

// GenerateFunc writes the CSS of node to g, as tokens, calling g.Node and
// g.Children for its child nodes.
type GenerateFunc func(g *Generator, node Node)

// WalkFieldsFunc calls walkNode for each non-nil node field and walkList
// for each non-nil list field of node, in structure order or, when reverse
// is set, in reverse structure order. It returns true as soon as walkNode
// or walkList does, which stops the walk.
type WalkFieldsFunc func(node Node, reverse bool, walkNode func(Node) bool, walkList func(*List[Node]) bool) bool

// CloneFunc returns a deep copy of node, of the same Go type, copying its
// child nodes with clone.
type CloneFunc func(node Node, clone func(Node) Node) Node

// FromPlainObjectFunc builds a node from its plain object form (see
// ToPlainObject). It reads the fields with the methods of r, which records
// the first error.
type FromPlainObjectFunc func(r *PlainObjectReader, object map[string]any) Node

// NodeConfig describes a node type to a syntax: its fields, how to
// generate it and how to walk, clone and decode it. Its parse function is
// not part of it; the built-in ones are methods of Parser, and an
// extension parses its nodes from a scope, context, at-rule or pseudo
// parser.
//
// The NodeConfigs of the node types of this package are generated from
// their struct tags. A node type added through Syntax.Fork writes its own;
// without Walk the walker does not descend into its nodes, without Clone
// Syntax.Clone shares them and without FromPlainObject
// Syntax.FromPlainObject rejects them. (Upstream reads node fields by name,
// so its node definitions need no such functions.)
type NodeConfig struct {
	// Name is the node type, as its Type method returns it.
	Name string
	// Structure lists the node's fields. Every node type needs one.
	Structure []StructureField
	// WalkContext names the WalkContext field that holds the nearest
	// enclosing node of this type during a walk, such as "rule"; "" for
	// none.
	WalkContext string
	// Generate writes the node's CSS.
	Generate GenerateFunc

	// Walk visits the node's child nodes and lists.
	Walk WalkFieldsFunc
	// Clone copies the node.
	Clone CloneFunc
	// FromPlainObject builds the node from its plain object form.
	FromPlainObject FromPlainObjectFunc
}

// structure field helpers

func stringField() FieldType  { return FieldType{Kind: FieldString} }
func booleanField() FieldType { return FieldType{Kind: FieldBoolean} }
func nullField() FieldType    { return FieldType{Kind: FieldNull} }

func nodeField(nodeType string) FieldType {
	return FieldType{Kind: FieldNode, Node: nodeType}
}

func listField(nodeTypes ...string) FieldType {
	return FieldType{Kind: FieldList, List: nodeTypes}
}

// JSON encoding

// jsonField is one field of a node's JSON object after "type" and "loc".
type jsonField struct {
	name  string
	value any
}

// marshalNode encodes a node as upstream JSON.stringify does: "type" first,
// then "loc", then fields in structure order.
func marshalNode(node Node, fields ...jsonField) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`{"type":`)
	if err := writeJSON(&buf, node.Type()); err != nil {
		return nil, err
	}
	buf.WriteString(`,"loc":`)
	if err := writeJSON(&buf, node.Location()); err != nil {
		return nil, err
	}
	for _, field := range fields {
		buf.WriteByte(',')
		if err := writeJSON(&buf, field.name); err != nil {
			return nil, err
		}
		buf.WriteByte(':')
		if err := writeJSON(&buf, field.value); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// writeJSON appends the JSON encoding of value without escaping <, > and &,
// as JSON.stringify does not.
func writeJSON(buf *bytes.Buffer, value any) error {
	if list, ok := value.(*List[Node]); ok && list != nil {
		// List.MarshalJSON uses json.Marshal, which escapes HTML
		buf.WriteByte('[')
		for item := list.Head(); item != nil; item = item.Next {
			if item != list.Head() {
				buf.WriteByte(',')
			}
			if err := writeJSON(buf, item.Data); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	}

	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1) // Encode appends a newline
	return nil
}

// nullableString returns nil for "" (upstream null) and s otherwise.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
