// Ported from lib/lexer/Lexer.js.

package csstree

import (
	"bytes"
	"errors"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/typelate/csstree/definitionsyntax"
)

// Errors the lexer's Match methods return for values they do not match at
// all.
var (
	// ErrVarNotSupported is returned for a value containing var(), which
	// the lexer cannot match.
	ErrVarNotSupported = errors.New("csstree: matching a value with var() is not supported")
	// ErrCustomProperty is returned for a custom property (--*), whose
	// value has no syntax to match.
	ErrCustomProperty = errors.New("csstree: custom property values are not matched")
	// ErrNotDeclaration is returned by MatchDeclaration for a node that is
	// not a *Declaration.
	ErrNotDeclaration = errors.New("csstree: not a Declaration node")
)

// AtruleSyntaxError reports an at-rule prelude or descriptor that the
// at-rule does not allow.
type AtruleSyntaxError struct {
	// Message describes the problem, such as "At-rule `@media` should
	// contain a prelude".
	Message string
}

// Error returns Message.
func (e *AtruleSyntaxError) Error() string {
	return e.Message
}

// MatchSyntax is the syntax a match tree node matched: a definition syntax
// node (definitionsyntax.Node: Type, Property, Keyword, ...), or a
// *SyntaxRef naming the syntax at the root of a match. SyntaxName returns
// its name.
type MatchSyntax interface {
	Type() string
}

// SyntaxKind is the kind of a syntax of the lexer.
type SyntaxKind string

// The kinds of syntax of the lexer.
const (
	KindType             SyntaxKind = "Type"
	KindProperty         SyntaxKind = "Property"
	KindAtrule           SyntaxKind = "Atrule"
	KindAtrulePrelude    SyntaxKind = "AtrulePrelude"
	KindAtruleDescriptor SyntaxKind = "AtruleDescriptor"
)

// SyntaxRef names a syntax of the lexer.
type SyntaxRef struct {
	// Kind is the kind of syntax.
	Kind SyntaxKind
	// Name is the name of the type, property or descriptor, or of the
	// at-rule for KindAtrule and KindAtrulePrelude.
	Name string
}

// Type returns Kind as a string, which is the Type of the definition
// syntax node of a Type or Property reference to the syntax.
func (r *SyntaxRef) Type() string {
	return string(r.Kind)
}

// SyntaxName returns the name of a match syntax: the Name of a *SyntaxRef
// or of a definition syntax Type, Property, Keyword, AtKeyword or Function
// node, and "" for other syntaxes.
func SyntaxName(syntax MatchSyntax) string {
	switch syntax := syntax.(type) {
	case *SyntaxRef:
		return syntax.Name
	case *definitionsyntax.Type:
		return syntax.Name
	case *definitionsyntax.Property:
		return syntax.Name
	case *definitionsyntax.Keyword:
		return syntax.Name
	case *definitionsyntax.AtKeyword:
		return syntax.Name
	case *definitionsyntax.Function:
		return syntax.Name
	}

	return ""
}

// MatchNode is a node of a match tree. A syntax node has a non-nil Match
// (possibly empty) holding what matched inside Syntax; a token node has a
// nil Match, the Token text and the AST Node it came from (nil for a
// string value or whitespace the generator added). Syntax is nil for
// tokens a generic type matched.
type MatchNode struct {
	// Syntax is the syntax this node matched.
	Syntax MatchSyntax
	// Match holds the nodes matched inside Syntax; nil for a token node.
	Match []*MatchNode
	// Token is the text of a token node.
	Token string
	// Node is the AST node a token node came from.
	Node Node
}

// MatchResult is the result of a match. The Match methods return a MatchResult even with an error,
// as io.Reader returns n with one, so that Iterations can be read; Matched
// is then nil. Matched is also nil, without an error, for an at-rule
// without a prelude matched against no prelude.
type MatchResult struct {
	// Matched is the root of the match tree.
	Matched *MatchNode
	// Iterations is the number of steps the matcher took.
	Iterations int
}

// Descriptor is a syntax of the lexer: a type, a property, an at-rule
// prelude or an at-rule descriptor (upstream createDescriptor's result).
// Its definition syntax is parsed and its match graph built on first use;
// a Descriptor is safe for concurrent use.
type Descriptor struct {
	kind SyntaxKind
	name string
	// parent is the at-rule name of an AtruleDescriptor, else "".
	parent string
	// serializable reports whether the syntax has a definition syntax
	// (false for generic types).
	serializable bool

	syntax   func() (definitionsyntax.Node, error)
	match    func() (*matchGraphNode, error)
	matchRef func() (*matchGraphNode, error)
}

// Kind returns the kind of syntax the descriptor is.
func (d *Descriptor) Kind() SyntaxKind {
	return d.kind
}

// Name returns the name of the type, property or at-rule descriptor, or of
// the at-rule of a prelude.
func (d *Descriptor) Name() string {
	return d.name
}

// Parent returns the at-rule name of a KindAtruleDescriptor, else "".
func (d *Descriptor) Parent() string {
	return d.parent
}

// Serializable reports whether the syntax has a definition syntax (false
// for generic types).
func (d *Descriptor) Serializable() bool {
	return d.serializable
}

// Syntax returns the definition syntax tree, parsing it on first use; it
// is nil for a generic type.
func (d *Descriptor) Syntax() (definitionsyntax.Node, error) {
	return d.syntax()
}

// AtruleSyntax is the syntax of an at-rule: its prelude and descriptors.
// It is safe for concurrent use.
type AtruleSyntax struct {
	name    string
	prelude *Descriptor
	// descriptors is nil when the at-rule has no known descriptors
	descriptors     map[string]*Descriptor
	descriptorNames []string
}

// Name returns the name of the at-rule.
func (a *AtruleSyntax) Name() string {
	return a.name
}

// Prelude returns the prelude syntax, nil when the at-rule has none.
func (a *AtruleSyntax) Prelude() *Descriptor {
	return a.prelude
}

// Descriptor returns the syntax of the descriptor name, nil when the
// at-rule has no such descriptor. The name must match exactly; the Match
// and Check methods of Lexer also accept a vendor-prefixed name.
func (a *AtruleSyntax) Descriptor(name string) *Descriptor {
	return a.descriptors[name]
}

// Descriptors returns the descriptors in name order.
func (a *AtruleSyntax) Descriptors() iter.Seq2[string, *Descriptor] {
	return func(yield func(string, *Descriptor) bool) {
		for _, name := range a.descriptorNames {
			if !yield(name, a.descriptors[name]) {
				return
			}
		}
	}
}

// LexerDump is the lexer's configuration with syntaxes as definition
// syntax text. Its JSON is that of upstream's dump().
type LexerDump struct {
	Generic         bool                        `json:"generic"`
	CSSWideKeywords []string                    `json:"cssWideKeywords"`
	Units           map[string][]string         `json:"units"`
	Types           map[string]string           `json:"types"`
	Properties      map[string]string           `json:"properties"`
	Atrules         map[string]AtruleDefinition `json:"atrules"`
}

// LexerValidation lists the broken syntaxes Validate found.
type LexerValidation struct {
	// Errors describe each reference to a missing or broken syntax.
	Errors []string `json:"errors"`
	// Types and Properties name the broken types and properties.
	Types      []string `json:"types"`
	Properties []string `json:"properties"`
}

func dumpMapSyntax(names []string, dict map[string]*Descriptor) (map[string]string, error) {
	result := make(map[string]string)

	for _, name := range names {
		syntax, err := dict[name].Syntax()
		if err != nil {
			return nil, err
		}
		if syntax != nil {
			result[name] = definitionsyntax.Generate(syntax, definitionsyntax.GenerateOptions{Compact: true})
		}
	}

	return result, nil
}

func dumpAtruleMapSyntax(names []string, dict map[string]*AtruleSyntax) (map[string]AtruleDefinition, error) {
	result := make(map[string]AtruleDefinition)

	for _, name := range names {
		atrule := dict[name]
		var definition AtruleDefinition

		if atrule.prelude != nil {
			syntax, err := atrule.prelude.Syntax()
			if err != nil {
				return nil, err
			}
			if syntax != nil {
				definition.Prelude = definitionsyntax.Generate(syntax, definitionsyntax.GenerateOptions{Compact: true})
			}
		}

		if atrule.descriptors != nil {
			descriptors, err := dumpMapSyntax(atrule.descriptorNames, atrule.descriptors)
			if err != nil {
				return nil, err
			}
			definition.Descriptors = descriptors
		}

		result[name] = definition
	}

	return result, nil
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

func valueHasVar(tokens []matchToken) bool {
	for i := 0; i < len(tokens); i++ {
		if toLowerCase(tokens[i].value) == "var(" || tokenNameIs(&tokens[i], "var(") {
			return true
		}
	}

	return false
}

func syntaxHasTopLevelCommaMultiplier(syntax definitionsyntax.Node) bool {
	group, ok := syntax.(*definitionsyntax.Group)
	if !ok || len(group.Terms) == 0 {
		return false
	}
	singleTerm, ok := group.Terms[0].(*definitionsyntax.Multiplier)

	return !group.Explicit &&
		len(group.Terms) == 1 &&
		ok &&
		singleTerm.Comma
}

func matchSyntax(lexer *Lexer, syntax *Descriptor, value matchInput, useCSSWideKeywords bool) (*MatchResult, error) {
	tokens, err := prepareTokens(value, lexer.syntax)
	if err != nil {
		return &MatchResult{}, err
	}
	var result *matchResult

	if valueHasVar(tokens) {
		return &MatchResult{}, ErrVarNotSupported
	}

	if useCSSWideKeywords {
		result, err = matchAsTree(tokens, lexer.cssWideKeywordsSyntax, lexer)
		if err != nil {
			return &MatchResult{}, err
		}
	}

	if !useCSSWideKeywords || result.tree == nil {
		match, err := syntax.match()
		if err != nil {
			return &MatchResult{}, err
		}
		result, err = matchAsTree(tokens, match, lexer)
		if err != nil {
			return &MatchResult{}, err
		}
		if result.tree == nil {
			definition, err := syntax.Syntax()
			if err != nil {
				return &MatchResult{}, err
			}
			return &MatchResult{Iterations: result.iterations}, newSyntaxMatchError(result.reason, definition, value.node, result)
		}
	}

	return &MatchResult{Matched: result.tree, Iterations: result.iterations}, nil
}

// Lexer matches CSS values against the syntaxes of types, properties and
// at-rules (upstream class Lexer). A Lexer does not change once created
// and is safe for concurrent use; for other syntaxes, create a lexer with
// Syntax.NewLexer or fork a syntax with Syntax.Fork.
//
// Values to match are ASTs, where a nil Node is no value; the methods with
// a String suffix take CSS text instead, which is tokenized rather than
// parsed. (Upstream takes either in the same argument.)
//
// The Match methods fail with a *SyntaxMatchError for a value that does
// not match (which unwraps to ErrIterationLimit when the matcher gave up),
// a *SyntaxReferenceError for an unknown type, property, at-rule or
// descriptor, or one a syntax refers to, an *AtruleSyntaxError for a
// prelude or descriptor the at-rule does not allow, ErrVarNotSupported,
// ErrCustomProperty or ErrNotDeclaration, an *UnknownNodeTypeError for a
// value they cannot generate, or the *definitionsyntax.SyntaxError of a
// syntax that does not parse.
type Lexer struct {
	cssWideKeywords []string
	syntax          *Syntax
	generic         bool
	units           map[string][]string
	atrules         map[string]*AtruleSyntax
	atruleNames     []string
	properties      map[string]*Descriptor
	propertyNames   []string
	types           map[string]*Descriptor
	typeNames       []string
	structure       map[string]*nodeStructure

	cssWideKeywordsSyntax *matchGraphNode
}

func newLexer(config *Config, syntax *Syntax, structure map[string]*nodeStructure) (*Lexer, error) {
	l := &Lexer{
		cssWideKeywords: cssWideKeywords(),
		syntax:          syntax,
		generic:         false,
		units:           defaultUnits(),
		atrules:         make(map[string]*AtruleSyntax),
		properties:      make(map[string]*Descriptor),
		types:           make(map[string]*Descriptor),
		structure:       structure,
	}
	if l.structure == nil {
		var err error
		l.structure, err = getStructureFromConfig(config)
		if err != nil {
			return nil, err
		}
	}

	if config != nil {
		if config.CSSWideKeywords != nil {
			l.cssWideKeywords = slices.Clone(config.CSSWideKeywords)
		}

		if config.Units != nil {
			for group := range unitGroupCount {
				if units, ok := config.Units[group.name()]; ok && units != nil {
					l.units[group.name()] = slices.Clone(units)
				}
			}
		}

		if config.Types != nil {
			for _, name := range sortedKeys(config.Types) {
				l.addType(name, config.Types[name])
			}
		}

		if config.Generic != nil && *config.Generic {
			l.generic = true
			for _, generic := range createGenericTypes(l.units) {
				l.addGenericType(generic)
			}
		}

		if config.Atrules != nil {
			for _, name := range sortedKeys(config.Atrules) {
				l.addAtrule(name, config.Atrules[name])
			}
		}

		if config.Properties != nil {
			for _, name := range sortedKeys(config.Properties) {
				l.addProperty(name, config.Properties[name])
			}
		}
	}

	cssWideKeywordsSyntax, err := buildMatchGraphFromString(strings.Join(l.cssWideKeywords, " |  "), nil)
	if err != nil {
		return nil, err
	}
	l.cssWideKeywordsSyntax = cssWideKeywordsSyntax

	return l, nil
}

// CheckStructure checks that every node of ast has the fields its node
// type's Structure describes, with values of the allowed kinds. It checks
// the plain object form of ast (see ToPlainObject), in which a node is a
// map and a list an array; the warnings refer to those maps. It returns
// nil when the structure is correct.
func (l *Lexer) CheckStructure(ast Node) ([]StructureWarning, error) {
	plain, err := l.syntax.ToPlainObject(ast)
	if err != nil {
		return nil, err
	}

	return l.CheckPlainObjectStructure(plain), nil
}

// CheckPlainObjectStructure is CheckStructure for an AST in its plain
// object form, such as JSON decoded into map[string]any.
func (l *Lexer) CheckPlainObjectStructure(ast map[string]any) []StructureWarning {
	structure := l.structure
	var warns []StructureWarning

	collectWarning := func(node any, message string) {
		warns = append(warns, StructureWarning{Node: node, Message: message})
	}

	l.walkPlainObject(ast, func(node any) {
		nodeType := "undefined"
		if object, ok := node.(map[string]any); ok {
			if typ, ok := object["type"]; ok {
				nodeType = jsString(typ)
			}
		}

		if nodeStructure, ok := structure[nodeType]; ok {
			nodeStructure.check(node, collectWarning)
		} else {
			collectWarning(node, "Unknown node type `"+nodeType+"`")
		}
	})

	return warns
}

// walkPlainObject calls enter for node and its descendants in the order
// the syntax's walker visits a parsed AST (upstream checkStructure uses
// syntax.walk, which walks plain objects too): the node and list fields of
// a node type's Structure, in order. Null fields and items are skipped.
func (l *Lexer) walkPlainObject(node any, enter func(node any)) {
	if node == nil {
		return
	}

	enter(node)

	object, ok := node.(map[string]any)
	if !ok {
		return
	}
	nodeType, ok := object["type"].(string)
	if !ok {
		return
	}

	for _, field := range l.syntax.config.Node[nodeType].Structure {
		isNode := false
		isList := false
		for _, fieldType := range field.Types {
			switch fieldType.Kind {
			case FieldNode:
				isNode = true
			case FieldList:
				isList = true
			}
		}

		ref := object[field.Name]
		switch {
		case isList:
			if items, ok := ref.([]any); ok {
				for _, item := range items {
					l.walkPlainObject(item, enter)
				}
			}
		case isNode:
			l.walkPlainObject(ref, enter)
		}
	}
}

// StructureDocs returns the allowed values of each field of a node type
// as text, such as "Identifier | null" (upstream
// lexer.structure[nodeType].docs), or nil for an unknown node type.
func (l *Lexer) StructureDocs(nodeType string) map[string]string {
	structure, ok := l.structure[nodeType]
	if !ok {
		return nil
	}

	return maps.Clone(structure.docs)
}

// descriptorSource is the syntax newDescriptor takes: definition syntax
// text, a definition syntax tree or a generic function (upstream a string,
// an object or a function).
type descriptorSource struct {
	text    string
	isText  bool
	tree    definitionsyntax.Node
	generic genericType
}

// newDescriptor is upstream createDescriptor; parent is the at-rule of a
// KindAtruleDescriptor.
func newDescriptor(source descriptorSource, ref SyntaxRef, parent string) *Descriptor {
	descriptor := &Descriptor{
		kind:         ref.Kind,
		name:         ref.Name,
		parent:       parent,
		serializable: source.isText || source.tree != nil,
	}

	if source.generic.fn != nil {
		descriptor.syntax = func() (definitionsyntax.Node, error) { return nil, nil }
		match := buildGenericMatchGraphRoot(source.generic, &ref)
		descriptor.match = func() (*matchGraphNode, error) { return match, nil }
		descriptor.matchRef = func() (*matchGraphNode, error) { return nil, nil }

		return descriptor
	}

	if source.isText {
		// lazy parsing on first access
		descriptor.syntax = sync.OnceValues(func() (definitionsyntax.Node, error) {
			group, err := definitionsyntax.Parse(source.text)
			if err != nil {
				return nil, err
			}
			return group, nil
		})
	} else {
		descriptor.syntax = func() (definitionsyntax.Node, error) { return source.tree, nil }
	}

	// lazy graph build on first access
	descriptor.match = sync.OnceValues(func() (*matchGraphNode, error) {
		syntax, err := descriptor.syntax()
		if err != nil {
			return nil, err
		}
		if syntax == nil {
			// upstream: buildMatchGraph(null) throws a TypeError
			return nil, &UnknownNodeTypeError{}
		}
		return buildMatchGraph(syntax, &ref)
	})

	descriptor.matchRef = func() (*matchGraphNode, error) { return nil, nil }
	if ref.Kind == KindProperty {
		descriptor.matchRef = sync.OnceValues(func() (*matchGraphNode, error) {
			syntax, err := descriptor.syntax()
			if err != nil {
				return nil, err
			}
			if !syntaxHasTopLevelCommaMultiplier(syntax) {
				return nil, nil
			}

			group := syntax.(*definitionsyntax.Group)
			return buildMatchGraph(&definitionsyntax.Group{
				Terms:         []definitionsyntax.Node{group.Terms[0].(*definitionsyntax.Multiplier).Term},
				Combinator:    group.Combinator,
				DisallowEmpty: group.DisallowEmpty,
				Explicit:      group.Explicit,
			}, &ref)
		})
	}

	return descriptor
}

// NewDescriptor returns a descriptor named ref for a definition syntax,
// such as "fn( <number># )", to match values against with any lexer's
// MatchDescriptor (upstream createDescriptor). The syntax is parsed on
// first use, so matching reports a syntax that does not parse. A
// descriptor keeps its parsed syntax and match graph, so matching many
// values against one descriptor is cheaper than Lexer.Match.
func NewDescriptor(ref SyntaxRef, syntax string) *Descriptor {
	return newDescriptor(descriptorSource{text: syntax, isText: true}, ref, "")
}

// NewDescriptorFromTree is NewDescriptor for a parsed definition syntax.
func NewDescriptorFromTree(ref SyntaxRef, syntax definitionsyntax.Node) *Descriptor {
	return newDescriptor(descriptorSource{tree: syntax}, ref, "")
}

// addAtrule adds or replaces the syntax of the at-rule name.
func (l *Lexer) addAtrule(name string, syntax AtruleDefinition) {
	atrule := &AtruleSyntax{
		name: name,
	}
	if syntax.Prelude != "" {
		atrule.prelude = NewDescriptor(SyntaxRef{Kind: KindAtrulePrelude, Name: name}, syntax.Prelude)
	}
	if syntax.Descriptors != nil {
		atrule.descriptors = make(map[string]*Descriptor)
		atrule.descriptorNames = sortedKeys(syntax.Descriptors)
		for _, descName := range atrule.descriptorNames {
			atrule.descriptors[descName] = newDescriptor(
				descriptorSourceFromText(syntax.Descriptors[descName]),
				SyntaxRef{Kind: KindAtruleDescriptor, Name: descName},
				name,
			)
		}
	}

	if _, ok := l.atrules[name]; !ok {
		l.atruleNames = append(l.atruleNames, name)
	}
	l.atrules[name] = atrule
}

// descriptorSourceFromText returns the source of a syntax; "" is upstream
// null, a descriptor without syntax.
func descriptorSourceFromText(syntax string) descriptorSource {
	if syntax == "" {
		return descriptorSource{}
	}
	return descriptorSource{text: syntax, isText: true}
}

// addProperty adds or replaces the syntax of the property name; an empty
// syntax is ignored.
func (l *Lexer) addProperty(name, syntax string) {
	if syntax == "" {
		return
	}

	if _, ok := l.properties[name]; !ok {
		l.propertyNames = append(l.propertyNames, name)
	}
	l.properties[name] = NewDescriptor(SyntaxRef{Kind: KindProperty, Name: name}, syntax)
}

// addType adds or replaces the syntax of the type name; an empty syntax
// is ignored.
func (l *Lexer) addType(name, syntax string) {
	if syntax == "" {
		return
	}

	l.setType(name, NewDescriptor(SyntaxRef{Kind: KindType, Name: name}, syntax))
}

func (l *Lexer) addGenericType(generic genericType) {
	l.setType(generic.name, newDescriptor(descriptorSource{generic: generic}, SyntaxRef{Kind: KindType, Name: generic.name}, ""))
}

func (l *Lexer) setType(name string, descriptor *Descriptor) {
	if _, ok := l.types[name]; !ok {
		l.typeNames = append(l.typeNames, name)
	}
	l.types[name] = descriptor
}

// CheckAtruleName returns a *SyntaxReferenceError when the at-rule is
// unknown, else nil.
func (l *Lexer) CheckAtruleName(atruleName string) error {
	if l.Atrule(atruleName) == nil {
		return newSyntaxReferenceError("Unknown at-rule", KindAtrule, "@"+atruleName)
	}

	return nil
}

// CheckAtrulePrelude checks that the at-rule exists and that prelude
// (nil for none) is present when, and only when, the at-rule requires
// one.
func (l *Lexer) CheckAtrulePrelude(atruleName string, prelude Node) error {
	return l.checkAtrulePrelude(atruleName, nodeInput(prelude))
}

// CheckAtrulePreludeString is CheckAtrulePrelude for prelude text ("" for
// none).
func (l *Lexer) CheckAtrulePreludeString(atruleName, prelude string) error {
	return l.checkAtrulePrelude(atruleName, cssInput(prelude))
}

func (l *Lexer) checkAtrulePrelude(atruleName string, prelude matchInput) error {
	if err := l.CheckAtruleName(atruleName); err != nil {
		return err
	}

	atrule := l.Atrule(atruleName)

	if atrule.prelude == nil && prelude.truthy() {
		return &AtruleSyntaxError{Message: "At-rule `@" + atruleName + "` should not contain a prelude"}
	}

	if atrule.prelude != nil && !prelude.truthy() {
		// upstream reads only the match: failing to match is a missing
		// prelude whatever the error
		result, err := matchSyntax(l, atrule.prelude, cssInput(""), false)
		if err != nil || result.Matched == nil {
			return &AtruleSyntaxError{Message: "At-rule `@" + atruleName + "` should contain a prelude"}
		}
	}

	return nil
}

// CheckAtruleDescriptorName checks that the at-rule exists and has the
// descriptor (with or without a vendor prefix).
func (l *Lexer) CheckAtruleDescriptorName(atruleName, descriptorName string) error {
	if err := l.CheckAtruleName(atruleName); err != nil {
		return err
	}

	atrule := l.Atrule(atruleName)
	descriptor := Keyword(descriptorName)

	if atrule.descriptors == nil {
		return &AtruleSyntaxError{Message: "At-rule `@" + atruleName + "` has no known descriptors"}
	}

	if atrule.descriptors[descriptor.Name] == nil &&
		atrule.descriptors[descriptor.Basename] == nil {
		return newSyntaxReferenceError("Unknown at-rule descriptor", KindAtruleDescriptor, descriptorName)
	}

	return nil
}

// CheckPropertyName returns a *SyntaxReferenceError when the property is
// unknown, else nil.
func (l *Lexer) CheckPropertyName(propertyName string) error {
	if l.Property(propertyName) == nil {
		return newSyntaxReferenceError("Unknown property", KindProperty, propertyName)
	}

	return nil
}

// MatchAtrulePrelude matches an at-rule prelude (nil for none) against the
// at-rule's prelude syntax.
func (l *Lexer) MatchAtrulePrelude(atruleName string, prelude Node) (*MatchResult, error) {
	return l.matchAtrulePrelude(atruleName, nodeInput(prelude))
}

// MatchAtrulePreludeString is MatchAtrulePrelude for prelude text ("" for
// none).
func (l *Lexer) MatchAtrulePreludeString(atruleName, prelude string) (*MatchResult, error) {
	return l.matchAtrulePrelude(atruleName, cssInput(prelude))
}

func (l *Lexer) matchAtrulePrelude(atruleName string, prelude matchInput) (*MatchResult, error) {
	if err := l.checkAtrulePrelude(atruleName, prelude); err != nil {
		return &MatchResult{}, err
	}

	atrule := l.Atrule(atruleName)

	if atrule.prelude == nil {
		return &MatchResult{}, nil
	}

	if !prelude.truthy() {
		prelude = cssInput("")
	}

	return matchSyntax(l, atrule.prelude, prelude, false)
}

// MatchAtruleDescriptor matches the value of an at-rule descriptor.
func (l *Lexer) MatchAtruleDescriptor(atruleName, descriptorName string, value Node) (*MatchResult, error) {
	return l.matchAtruleDescriptor(atruleName, descriptorName, nodeInput(value))
}

// MatchAtruleDescriptorString is MatchAtruleDescriptor for value text.
func (l *Lexer) MatchAtruleDescriptorString(atruleName, descriptorName, value string) (*MatchResult, error) {
	return l.matchAtruleDescriptor(atruleName, descriptorName, cssInput(value))
}

func (l *Lexer) matchAtruleDescriptor(atruleName, descriptorName string, value matchInput) (*MatchResult, error) {
	if err := l.CheckAtruleDescriptorName(atruleName, descriptorName); err != nil {
		return &MatchResult{}, err
	}

	atrule := l.Atrule(atruleName)
	descriptor := Keyword(descriptorName)

	syntax := atrule.descriptors[descriptor.Name]
	if syntax == nil {
		syntax = atrule.descriptors[descriptor.Basename]
	}

	return matchSyntax(l, syntax, value, false)
}

// MatchDeclaration matches the value of a Declaration node against its
// property's syntax.
func (l *Lexer) MatchDeclaration(node Node) (*MatchResult, error) {
	declaration, ok := node.(*Declaration)
	if !ok {
		return &MatchResult{}, ErrNotDeclaration
	}

	return l.MatchProperty(declaration.Property, declaration.Value)
}

// MatchProperty matches value against the syntax of the property, also
// accepting the CSS-wide keywords.
func (l *Lexer) MatchProperty(propertyName string, value Node) (*MatchResult, error) {
	return l.matchProperty(propertyName, nodeInput(value))
}

// MatchPropertyString is MatchProperty for value text.
func (l *Lexer) MatchPropertyString(propertyName, value string) (*MatchResult, error) {
	return l.matchProperty(propertyName, cssInput(value))
}

func (l *Lexer) matchProperty(propertyName string, value matchInput) (*MatchResult, error) {
	// don't match syntax for a custom property at the moment
	if Property(propertyName).Custom {
		return &MatchResult{}, ErrCustomProperty
	}

	if err := l.CheckPropertyName(propertyName); err != nil {
		return &MatchResult{}, err
	}

	return matchSyntax(l, l.Property(propertyName), value, true)
}

// MatchType matches value against the syntax of the type typeName.
func (l *Lexer) MatchType(typeName string, value Node) (*MatchResult, error) {
	return l.matchType(typeName, nodeInput(value))
}

// MatchTypeString is MatchType for value text.
func (l *Lexer) MatchTypeString(typeName, value string) (*MatchResult, error) {
	return l.matchType(typeName, cssInput(value))
}

func (l *Lexer) matchType(typeName string, value matchInput) (*MatchResult, error) {
	typeSyntax := l.Type(typeName)

	if typeSyntax == nil {
		return &MatchResult{}, newSyntaxReferenceError("Unknown type", KindType, typeName)
	}

	return matchSyntax(l, typeSyntax, value, false)
}

// anonymousRef names the syntaxes of Match and MatchString.
var anonymousRef = SyntaxRef{Kind: KindType, Name: "anonymous"}

// Match matches value against a definition syntax, such as
// "fn( <number># )" (upstream match with a string syntax). It parses the
// syntax and builds its match graph on every call; to match many values
// against one syntax, create a descriptor with NewDescriptor once and use
// MatchDescriptor.
func (l *Lexer) Match(syntax string, value Node) (*MatchResult, error) {
	return l.MatchDescriptor(NewDescriptor(anonymousRef, syntax), value)
}

// MatchString is Match for value text.
func (l *Lexer) MatchString(syntax, value string) (*MatchResult, error) {
	return l.MatchDescriptorString(NewDescriptor(anonymousRef, syntax), value)
}

// MatchDescriptor matches value against a descriptor, such as one Type
// returns (upstream match with a descriptor). A nil descriptor is a "Bad
// syntax" *SyntaxReferenceError.
func (l *Lexer) MatchDescriptor(syntax *Descriptor, value Node) (*MatchResult, error) {
	return l.match(syntax, nodeInput(value))
}

// MatchDescriptorString is MatchDescriptor for value text.
func (l *Lexer) MatchDescriptorString(syntax *Descriptor, value string) (*MatchResult, error) {
	return l.match(syntax, cssInput(value))
}

func (l *Lexer) match(syntax *Descriptor, value matchInput) (*MatchResult, error) {
	if syntax == nil {
		return &MatchResult{}, newSyntaxReferenceError("Bad syntax", "", "")
	}

	return matchSyntax(l, syntax, value, false)
}

// FindValueFragments returns the runs of nodes of value that matched the
// syntax target when matching value against the property's syntax. The
// syntaxes of the match compare by Type and SyntaxName: target.Kind is
// KindType or KindProperty for a type or property, or the Type of another
// definition syntax node, such as "Keyword" or "Function". A value that
// does not match has no fragments.
func (l *Lexer) FindValueFragments(propertyName string, value Node, target SyntaxRef) ([]Fragment, error) {
	match, err := l.MatchProperty(propertyName, value)
	if err != nil {
		// a value that does not match has no fragments
		return nil, nil
	}

	return matchFragments(l, value, match, target)
}

// FindDeclarationValueFragments is FindValueFragments for the property
// and value of a Declaration node.
func (l *Lexer) FindDeclarationValueFragments(declaration Node, target SyntaxRef) ([]Fragment, error) {
	var value Node
	if declaration, ok := declaration.(*Declaration); ok {
		value = declaration.Value
	}

	match, err := l.MatchDeclaration(declaration)
	if err != nil {
		// a value that does not match has no fragments
		return nil, nil
	}

	return matchFragments(l, value, match, target)
}

// FindAllFragments is FindDeclarationValueFragments for every declaration
// in ast.
func (l *Lexer) FindAllFragments(ast Node, target SyntaxRef) ([]Fragment, error) {
	var result []Fragment
	var findErr error

	err := l.syntax.walkReadOnly(ast, WalkOptions{
		Visit: "Declaration",
		Enter: func(_ *WalkContext, declaration Node, _ *ListItem[Node], _ *List[Node]) WalkAction {
			fragments, err := l.FindDeclarationValueFragments(declaration, target)
			if err != nil {
				findErr = err
				return WalkBreak
			}
			result = append(result, fragments...)
			return WalkContinue
		},
	})
	if err != nil {
		return nil, err
	}
	if findErr != nil {
		return nil, findErr
	}

	return result, nil
}

// Atrule returns the syntax of an at-rule, nil when it is unknown. The name
// is ASCII case-insensitive, and a vendor-prefixed name falls back to the
// name without the prefix (upstream getAtrule(name, true)).
func (l *Lexer) Atrule(name string) *AtruleSyntax {
	keyword := Keyword(name)
	if atrule := l.atrules[keyword.Name]; atrule != nil {
		return atrule
	}

	return l.atrules[keyword.Basename]
}

// AtrulePrelude returns the prelude syntax of an at-rule, nil when the
// at-rule is unknown or has no prelude. The name is looked up as Atrule
// does.
func (l *Lexer) AtrulePrelude(atruleName string) *Descriptor {
	atrule := l.Atrule(atruleName)
	if atrule == nil {
		return nil
	}

	return atrule.prelude
}

// Property returns the syntax of a property, nil when it is unknown. The
// name is ASCII case-insensitive, may have a hack prefix (such as "_" or
// "*"), and a vendor-prefixed name falls back to the name without the
// prefix (upstream getProperty(name, true)).
func (l *Lexer) Property(name string) *Descriptor {
	property := Property(name)
	if descriptor := l.properties[property.Name]; descriptor != nil {
		return descriptor
	}

	return l.properties[property.Basename]
}

// Type returns the syntax of a type, nil when it is unknown.
func (l *Lexer) Type(name string) *Descriptor {
	return l.types[name]
}

// Validate checks that every syntax refers only to types and properties
// that exist and are valid themselves. It returns nil when all are valid,
// and an error when a syntax does not parse.
func (l *Lexer) Validate() (*LexerValidation, error) {
	syntaxRef := func(name string, isType bool) string {
		if isType {
			return "<" + name + ">"
		}
		return "<'" + name + "'>"
	}

	var errs []string
	brokenTypes := newBrokenMap()
	brokenProperties := newBrokenMap()

	var validate func(name string, broken *brokenMap, descriptor *Descriptor) (bool, error)
	validate = func(name string, broken *brokenMap, descriptor *Descriptor) (bool, error) {
		if value, ok := broken.get(name); ok {
			return value, nil
		}

		broken.set(name, false)
		syntax, err := descriptor.Syntax()
		if err != nil {
			return false, err
		}
		if syntax != nil {
			var walkErr error
			err := definitionsyntax.Walk(syntax, definitionsyntax.WalkOptions{
				Enter: func(node definitionsyntax.Node) {
					if walkErr != nil {
						return
					}

					var nodeName string
					var isType bool
					switch node := node.(type) {
					case *definitionsyntax.Type:
						nodeName = node.Name
						isType = true
					case *definitionsyntax.Property:
						nodeName = node.Name
					default:
						return
					}

					dict := l.properties
					brokenDict := brokenProperties
					if isType {
						dict = l.types
						brokenDict = brokenTypes
					}

					if _, ok := dict[nodeName]; !ok {
						errs = append(errs, syntaxRef(name, broken == brokenTypes)+" used missed syntax definition "+syntaxRef(nodeName, isType))
						broken.set(name, true)
					} else {
						isBroken, err := validate(nodeName, brokenDict, dict[nodeName])
						if err != nil {
							walkErr = err
							return
						}
						if isBroken {
							errs = append(errs, syntaxRef(name, broken == brokenTypes)+" used broken syntax definition "+syntaxRef(nodeName, isType))
							broken.set(name, true)
						}
					}
				},
			})
			if err != nil {
				return false, err
			}
			if walkErr != nil {
				return false, walkErr
			}
		}

		// upstream returns undefined here: a name being validated is not
		// (yet) broken for the syntaxes that refer to it
		return false, nil
	}

	for _, key := range l.typeNames {
		if _, err := validate(key, brokenTypes, l.types[key]); err != nil {
			return nil, err
		}
	}

	for _, key := range l.propertyNames {
		if _, err := validate(key, brokenProperties, l.properties[key]); err != nil {
			return nil, err
		}
	}

	brokenTypesArray := brokenTypes.brokenNames()
	brokenPropertiesArray := brokenProperties.brokenNames()

	if len(brokenTypesArray) > 0 || len(brokenPropertiesArray) > 0 {
		return &LexerValidation{
			Errors:     errs,
			Types:      brokenTypesArray,
			Properties: brokenPropertiesArray,
		}, nil
	}

	return nil, nil
}

// brokenMap is the Map Validate keeps per dictionary: names in insertion
// order with whether they are broken.
type brokenMap struct {
	names  []string
	broken map[string]bool
}

func newBrokenMap() *brokenMap {
	return &brokenMap{broken: make(map[string]bool)}
}

func (m *brokenMap) get(name string) (bool, bool) {
	value, ok := m.broken[name]
	return value, ok
}

func (m *brokenMap) set(name string, value bool) {
	if _, ok := m.broken[name]; !ok {
		m.names = append(m.names, name)
	}
	m.broken[name] = value
}

func (m *brokenMap) brokenNames() []string {
	var result []string
	for _, name := range m.names {
		if m.broken[name] {
			result = append(result, name)
		}
	}
	return result
}

// Dump returns the lexer's configuration with its syntaxes generated as
// compact definition syntax text (upstream dump()). Generic types, which
// have no definition syntax, are left out. It fails when a syntax does not
// parse. For other text, generate the trees of the descriptors of Types,
// Properties and Atrules with definitionsyntax.Generate.
func (l *Lexer) Dump() (*LexerDump, error) {
	types, err := dumpMapSyntax(l.typeNames, l.types)
	if err != nil {
		return nil, err
	}
	properties, err := dumpMapSyntax(l.propertyNames, l.properties)
	if err != nil {
		return nil, err
	}
	atrules, err := dumpAtruleMapSyntax(l.atruleNames, l.atrules)
	if err != nil {
		return nil, err
	}

	return &LexerDump{
		Generic:         l.generic,
		CSSWideKeywords: slices.Clone(l.cssWideKeywords),
		Units:           l.Units(),
		Types:           types,
		Properties:      properties,
		Atrules:         atrules,
	}, nil
}

// String returns the JSON of Dump (upstream toString), or the error text
// when a syntax does not parse.
func (l *Lexer) String() string {
	dump, err := l.Dump()
	if err != nil {
		return err.Error()
	}

	var buf bytes.Buffer
	if err := writeJSON(&buf, dump); err != nil {
		return err.Error()
	}

	return buf.String()
}

// Generic reports whether the lexer has the generic types (upstream
// lexer.generic).
func (l *Lexer) Generic() bool {
	return l.generic
}

// CSSWideKeywords returns the CSS-wide keywords every property accepts.
func (l *Lexer) CSSWideKeywords() []string {
	return slices.Clone(l.cssWideKeywords)
}

// Units returns the units of each unit group (upstream lexer.units).
func (l *Lexer) Units() map[string][]string {
	units := make(map[string][]string, len(l.units))
	for group, list := range l.units {
		units[group] = slices.Clone(list)
	}
	return units
}

// Types returns the types in the order they were added.
func (l *Lexer) Types() iter.Seq2[string, *Descriptor] {
	return func(yield func(string, *Descriptor) bool) {
		for _, name := range l.typeNames {
			if !yield(name, l.types[name]) {
				return
			}
		}
	}
}

// Properties returns the properties in the order they were added.
func (l *Lexer) Properties() iter.Seq2[string, *Descriptor] {
	return func(yield func(string, *Descriptor) bool) {
		for _, name := range l.propertyNames {
			if !yield(name, l.properties[name]) {
				return
			}
		}
	}
}

// Atrules returns the at-rules in the order they were added.
func (l *Lexer) Atrules() iter.Seq2[string, *AtruleSyntax] {
	return func(yield func(string, *AtruleSyntax) bool) {
		for _, name := range l.atruleNames {
			if !yield(name, l.atrules[name]) {
				return
			}
		}
	}
}

// lookupMatch implements matchSyntaxes.
func (l *Lexer) lookupMatch(isType bool, name string) (*matchGraphNode, error) {
	dict := l.properties
	if isType {
		dict = l.types
	}

	dictSyntax, ok := dict[name]
	if !ok || dictSyntax == nil {
		return nil, nil
	}

	match, err := dictSyntax.match()
	if err != nil || match == nil {
		return nil, err
	}

	matchRef, err := dictSyntax.matchRef()
	if err != nil {
		return nil, err
	}
	if matchRef != nil {
		return matchRef, nil
	}

	return match, nil
}
