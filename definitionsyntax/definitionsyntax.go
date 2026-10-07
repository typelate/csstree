// Ported from lib/definition-syntax/index.js. The node types mirror the
// object literals built in lib/definition-syntax/parse.js.

// Package definitionsyntax parses, generates and walks CSS value definition
// syntax, such as `<length> | auto`, the grammar the CSS specifications use
// to describe property values (CSS Values 4 §2). It is ported from
// csstree's lib/definition-syntax; package csstree's lexer matches values
// against the trees it parses.
//
// [Parse] returns the syntax as a [*Group] of terms joined by a
// [Combinator]: [Juxtaposition], [DoubleAmpersand], [DoubleBar] or
// [SingleBar]. A [Multiplier] repeats a term from Min to Max times; Max is
// [Unbounded] for `*`, `+`, `#` and `{A,}`, and `{0}` means exactly zero
// times. A malformed syntax gives a [*SyntaxError].
//
// [Generate] turns a tree back into text, and [Walk] visits its nodes. The
// set of node types is closed: [Node] is implemented only by the pointer
// types of this package. Walk returns [ErrNilNode] when it meets a nil
// node. Nodes encode to JSON as upstream's syntax trees do, except for an
// exact zero Max (see [Multiplier.MarshalJSON]).
package definitionsyntax

import (
	"encoding/json"
	"math"

	"github.com/typelate/csstree/internal/jsnum"
)

// Node is a definition syntax AST node. The set of node types is closed: it
// is implemented only by the pointer types in this package.
type Node interface {
	// Type returns the upstream node.type, e.g. "Group".
	Type() string

	node()
}

// Combinator is how the terms of a Group combine (CSS Values 4 §2.2,
// Component Value Combinators). Its value is the upstream string.
type Combinator string

// The combinators, from the tightest binding to the loosest.
const (
	// Juxtaposition: all terms must occur, in the given order.
	Juxtaposition Combinator = " "
	// DoubleAmpersand: all terms must occur, in any order.
	DoubleAmpersand Combinator = "&&"
	// DoubleBar: one or more terms must occur, in any order.
	DoubleBar Combinator = "||"
	// SingleBar: exactly one term must occur.
	SingleBar Combinator = "|"
)

// Group is a sequence of terms joined by one combinator.
type Group struct {
	Terms         []Node
	Combinator    Combinator
	DisallowEmpty bool // the group is followed by "!"
	Explicit      bool // the group is written in square brackets
}

// Unbounded is the Max of a Multiplier with no upper bound, such as `*`, `+`,
// `#` or `{A,}`.
const Unbounded = -1

// Multiplier repeats Term from Min to Max times. Max is Unbounded when there
// is no upper bound; a Max of 0, as in `{0}`, means exactly zero times
// (upstream uses max 0 for "no upper bound", so it reads `{0}` as `*`).
// Comma is set for the "#" multiplier.
type Multiplier struct {
	Comma bool
	Min   int
	Max   int
	Term  Node
}

// Boolean is a `<boolean-expr[ ... ]>` type; Term is the test inside the brackets.
type Boolean struct {
	Term Node
}

// Type is a reference to a type, such as `<length>`. Opts is nil unless the
// reference has a bracketed range, such as `<length [0,∞]>`.
type Type struct {
	Name string
	Opts *Range
}

// Range is a bracketed range restriction on a numeric Type. A nil Min or Max
// is an infinite end (upstream null).
type Range struct {
	Min *RangeBound
	Max *RangeBound
}

// RangeBound is a finite end of a Range. Upstream holds a number, or a string
// such as "0s" when a unit follows the number; Unit is that suffix.
type RangeBound struct {
	Value float64
	Unit  string
}

// Property is a reference to a property's value syntax, such as `<'width'>`.
type Property struct {
	Name string
}

// Keyword is an identifier such as `auto`.
type Keyword struct {
	Name string
}

// AtKeyword is an at-keyword such as `@media`; Name has no "@".
type AtKeyword struct {
	Name string
}

// Function is a function name with its opening parenthesis, such as `rgb(`;
// Name has no "(". The function's arguments and closing parenthesis follow it
// as sibling terms.
type Function struct {
	Name string
}

// String is a quoted string such as `'/'`; Value includes the quotes.
type String struct {
	Value string
}

// Token is a single character, such as `/` or `)`.
type Token struct {
	Value string
}

// Comma is a `,`.
type Comma struct{}

func (*Group) Type() string      { return "Group" }
func (*Multiplier) Type() string { return "Multiplier" }
func (*Boolean) Type() string    { return "Boolean" }
func (*Type) Type() string       { return "Type" }
func (*Range) Type() string      { return "Range" }
func (*Property) Type() string   { return "Property" }
func (*Keyword) Type() string    { return "Keyword" }
func (*AtKeyword) Type() string  { return "AtKeyword" }
func (*Function) Type() string   { return "Function" }
func (*String) Type() string     { return "String" }
func (*Token) Type() string      { return "Token" }
func (*Comma) Type() string      { return "Comma" }

func (*Group) node()      {}
func (*Multiplier) node() {}
func (*Boolean) node()    {}
func (*Type) node()       {}
func (*Range) node()      {}
func (*Property) node()   {}
func (*Keyword) node()    {}
func (*AtKeyword) node()  {}
func (*Function) node()   {}
func (*String) node()     {}
func (*Token) node()      {}
func (*Comma) node()      {}

// String formats b as JavaScript's String(value) does upstream.
func (b RangeBound) String() string {
	return formatNumber(b.Value) + b.Unit
}

// formatNumber formats v as JavaScript's Number.prototype.toString does
// (ECMA-262 Number::toString): the shortest round-tripping digits, written
// in plain notation for decimal exponents in [-6, 21) and as d.ddde±x
// otherwise.
func formatNumber(v float64) string {
	return jsnum.Format(v)
}

// The MarshalJSON methods match upstream JSON.stringify(ast): "type" first,
// then the fields in the order parse.js creates them.

func (n *Group) MarshalJSON() ([]byte, error) {
	terms := n.Terms
	if terms == nil {
		terms = []Node{}
	}
	return json.Marshal(struct {
		Type          string     `json:"type"`
		Terms         []Node     `json:"terms"`
		Combinator    Combinator `json:"combinator"`
		DisallowEmpty bool       `json:"disallowEmpty"`
		Explicit      bool       `json:"explicit"`
	}{n.Type(), terms, n.Combinator, n.DisallowEmpty, n.Explicit})
}

// MarshalJSON writes max 0 for Unbounded, as upstream does. An exact zero
// max, which upstream cannot represent, is written as null: upstream's
// matcher reads a null max as no repetitions, never as unbounded.
func (n *Multiplier) MarshalJSON() ([]byte, error) {
	var maxValue *int
	switch n.Max {
	case Unbounded:
		maxValue = new(0)
	case 0:
		// null
	default:
		maxValue = new(n.Max)
	}
	return json.Marshal(struct {
		Type  string `json:"type"`
		Comma bool   `json:"comma"`
		Min   int    `json:"min"`
		Max   *int   `json:"max"`
		Term  Node   `json:"term"`
	}{n.Type(), n.Comma, n.Min, maxValue, n.Term})
}

func (n *Boolean) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		Term Node   `json:"term"`
	}{n.Type(), n.Term})
}

func (n *Type) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Opts *Range `json:"opts"`
	}{n.Type(), n.Name, n.Opts})
}

func (n *Range) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string      `json:"type"`
		Min  *RangeBound `json:"min"`
		Max  *RangeBound `json:"max"`
	}{n.Type(), n.Min, n.Max})
}

// MarshalJSON encodes b as a JSON number, or as a string when it has a unit.
func (b RangeBound) MarshalJSON() ([]byte, error) {
	if b.Unit != "" {
		return json.Marshal(b.String())
	}
	if math.IsInf(b.Value, 0) {
		return []byte("null"), nil // JSON.stringify(Infinity)
	}
	return []byte(formatNumber(b.Value)), nil
}

func (n *Property) MarshalJSON() ([]byte, error) {
	return marshalName(n.Type(), n.Name)
}

func (n *Keyword) MarshalJSON() ([]byte, error) {
	return marshalName(n.Type(), n.Name)
}

func (n *AtKeyword) MarshalJSON() ([]byte, error) {
	return marshalName(n.Type(), n.Name)
}

func (n *Function) MarshalJSON() ([]byte, error) {
	return marshalName(n.Type(), n.Name)
}

func (n *String) MarshalJSON() ([]byte, error) {
	return marshalValue(n.Type(), n.Value)
}

func (n *Token) MarshalJSON() ([]byte, error) {
	return marshalValue(n.Type(), n.Value)
}

func (n *Comma) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
	}{n.Type()})
}

func marshalName(nodeType, name string) ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}{nodeType, name})
}

func marshalValue(nodeType, value string) ([]byte, error) {
	return json.Marshal(struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}{nodeType, value})
}
