// Ported from lib/syntax/create.js, lib/syntax/index.js and lib/index.js.

package csstree

import "sync"

// Syntax is a CSS syntax: the parser, generator, walker, convertor and
// lexer of one configuration (upstream the object createSyntax returns).
// A Syntax is safe for concurrent use. An AST may be read by several
// goroutines at once (Generate, Find, FindLast, FindAll, Clone,
// ToPlainObject, lexer matching), but Walk and any change to the AST need
// exclusive access to it.
type Syntax struct {
	lexer     *Lexer
	config    *Config
	parse     parseFunc
	walker    *walker
	generate  generateFunc
	convertor *convertor
	clone     func(node Node) Node
}

// SyntaxExtension is what Fork mixes into a copy of a syntax's config:
// a *Config, whose set fields are mixed in as upstream mix does, or an
// ExtendFunc, which changes the copy and returns the config to use
// (upstream fork(extension) takes an object or a function).
type SyntaxExtension interface {
	extend(base *Config) *Config
}

func (c *Config) extend(base *Config) *Config {
	return mix(base, c)
}

// ExtendFunc is the function form of a SyntaxExtension: it receives a
// copy of the syntax's config, which it may change, and returns the
// config of the new syntax.
type ExtendFunc func(base *Config) *Config

func (fn ExtendFunc) extend(base *Config) *Config {
	return fn(base)
}

func createSyntax(config *Config) (*Syntax, error) {
	if config == nil {
		config = new(Config)
	}
	parse := createParser(config)
	walker, err := createWalker(config)
	if err != nil {
		return nil, err
	}
	generate := createGenerator(config)
	convertor := createConvertor(config)

	syntax := &Syntax{
		config:    config,
		parse:     parse,
		walker:    walker,
		generate:  generate,
		convertor: convertor,
		clone:     cloneWith(cloneTypesFromConfig(config)),
	}

	lexer, err := newLexer(&Config{
		Generic:         config.Generic,
		CSSWideKeywords: config.CSSWideKeywords,
		Units:           config.Units,
		Types:           config.Types,
		Atrules:         config.Atrules,
		Properties:      config.Properties,
		Node:            config.Node,
	}, syntax, nil)
	if err != nil {
		return nil, err
	}
	syntax.lexer = lexer

	return syntax, nil
}

// Lexer returns the lexer of the syntax (upstream syntax.lexer): the lexer
// of the lexer keys of its config (Generic, CSSWideKeywords, Units, Types,
// Atrules, Properties, Node).
func (s *Syntax) Lexer() *Lexer {
	return s.lexer
}

// CreateLexer returns a new lexer for the lexer keys of config, sharing
// this syntax and its node structures (upstream createLexer). It fails when
// the CSS-wide keywords do not parse as a definition syntax.
func (s *Syntax) CreateLexer(config *Config) (*Lexer, error) {
	return newLexer(config, s, s.lexer.structure)
}

// NewSyntax returns the syntax of a copy of config (upstream
// createSyntax). It fails when a node type has no Structure or a bad one,
// or when the CSS-wide keywords do not parse as a definition syntax.
func NewSyntax(config *Config) (*Syntax, error) {
	return createSyntax(mix(nil, config))
}

// Parse parses source into an AST, whose root is the node of the parse
// context options.Context. It fails with an *UnknownContextError for a
// context the syntax does not define, with a *SyntaxError the parser does
// not recover from, with the error OnParseError returns, or with an
// *InternalError when a parse function or callback panics.
func (s *Syntax) Parse(source string, options ParseOptions) (Node, error) {
	return s.parse(source, options)
}

// Generate turns an AST into CSS text. It fails with an
// *UnknownNodeTypeError for a node (or a nil node) whose type has no
// generate function, or with an *InternalError when a generate function
// panics, e.g. on a hand-built node missing a required field.
func (s *Syntax) Generate(node Node, options GenerateOptions) (string, error) {
	return s.generate(node, options.Mode, nil)
}

// GenerateWithSourceMap is Generate also returning a source map of the
// generated CSS (upstream generate with sourceMap: true, which returns
// { css, map }). Only Atrule, Selector and Declaration nodes with a
// location (see ParseOptions.Positions) are mapped. It also fails when such
// a location is not a valid source position (a Line below 1, a Column below
// 1 or an empty Filename).
func (s *Syntax) GenerateWithSourceMap(node Node, options GenerateOptions) (css string, sourceMap *SourceMapGenerator, err error) {
	sourceMap = new(SourceMapGenerator)
	css, err = s.generate(node, options.Mode, sourceMapDecorator(sourceMap))
	if err != nil {
		return "", nil, err
	}
	return css, sourceMap, nil
}

// Walk traverses root depth-first, calling options.Enter before and
// options.Leave after the children of each node. It fails when neither
// handler is set or options.Visit names an unknown node type. Nil nodes
// (a nil root or nil list data) are not visited.
//
// The handlers may change the list holding the current node (insert or
// remove items, as List.ForEach allows), which is why Walk records its
// position in each list it walks: it must not run concurrently with any
// other use of the same AST, not even a read-only one such as Generate or
// Find.
func (s *Syntax) Walk(root Node, options WalkOptions) error {
	return s.walker.Walk(root, options)
}

// walkReadOnly is Walk for handlers that do not change the AST: it does not
// write to the lists it walks, so it may run concurrently with other
// read-only uses of the AST.
func (s *Syntax) walkReadOnly(root Node, options WalkOptions) error {
	return s.walker.walk(root, options, false)
}

// Find returns the first node, in natural walk order, for which fn returns
// true, or nil. fn must not change the AST: Find, FindLast and FindAll only
// read it, so they may run concurrently with other reads of the same AST.
func (s *Syntax) Find(ast Node, fn FindFunc) Node {
	return s.walker.Find(ast, fn)
}

// FindLast returns the first node, in reverse walk order, for which fn
// returns true, or nil (see Find).
func (s *Syntax) FindLast(ast Node, fn FindFunc) Node {
	return s.walker.FindLast(ast, fn)
}

// FindAll returns every node, in natural walk order, for which fn returns
// true (see Find).
func (s *Syntax) FindAll(ast Node, fn FindFunc) []Node {
	return s.walker.FindAll(ast, fn)
}

// FromPlainObject builds an AST from its plain object form, such as the
// result of ToPlainObject or of decoding the JSON of an AST into an any. It
// fails on a value of the wrong kind or a node of a type without a
// FromPlainObject function.
func (s *Syntax) FromPlainObject(ast map[string]any) (Node, error) {
	return s.convertor.FromPlainObject(ast)
}

// ToPlainObject returns the plain object form of ast: the value of
// JSON.parse(JSON.stringify(ast)), a map per node with "type", "loc" and
// the node fields, and a slice per list. Nodes are encoded with
// encoding/json, so a node type added through Fork needs a MarshalJSON
// method writing "type".
func (s *Syntax) ToPlainObject(ast Node) (map[string]any, error) {
	return s.convertor.ToPlainObject(ast)
}

// Clone returns a deep copy of node, copying nodes of every type of the
// syntax that has a Clone function (Go only; see Clone).
func (s *Syntax) Clone(node Node) Node {
	return s.clone(node)
}

// Fork returns a new syntax whose config is a copy of this syntax's
// config extended by extension (a *Config or an ExtendFunc). The syntax
// it is forked from is not affected. A nil extension forks an unchanged
// copy.
func (s *Syntax) Fork(extension SyntaxExtension) (*Syntax, error) {
	base := mix(nil, s.config) // copy of config

	if extension == nil {
		return createSyntax(base)
	}

	return createSyntax(extension.extend(base))
}

// defaultSyntax is upstream lib/syntax/index.js:
// createSyntax({ ...lexerConfig, ...parserConfig, ...walkerConfig }).
var defaultSyntax = sync.OnceValue(func() *Syntax {
	lexer, err := lexerConfig()
	if err != nil {
		// the embedded data is decoded by the tests
		panic("csstree: default syntax: " + err.Error())
	}

	return createDefaultSyntax(lexer)
})

// defaultParseSyntax is defaultSyntax without the definitions of data.go
// (types, properties, at-rules): the package-level Parse, Generate, Walk,
// Find, FindLast, FindAll, ToPlainObject and FromPlainObject use
// it, so that a program that only parses does not decode mdn-data. It
// parses, generates and walks exactly as defaultSyntax does.
var defaultParseSyntax = sync.OnceValue(func() *Syntax {
	return createDefaultSyntax(lexerConfigWithoutDefinitions())
})

func createDefaultSyntax(lexer *Config) *Syntax {
	parser := parserConfig()
	walker := walkerConfig()

	syntax, err := NewSyntax(&Config{
		Contexts: parser.Contexts,
		Features: parser.Features,
		Scope:    parser.Scope,
		Atrule:   parser.Atrule,
		Pseudo:   parser.Pseudo,
		Node:     walker.Node,

		Generic:         lexer.Generic,
		CSSWideKeywords: lexer.CSSWideKeywords,
		Units:           lexer.Units,
		Types:           lexer.Types,
		Atrules:         lexer.Atrules,
		Properties:      lexer.Properties,
	})
	if err != nil {
		// every node type of this package has a valid structure
		panic("csstree: default syntax: " + err.Error())
	}

	return syntax
}

// DefaultSyntax returns the default syntax, with the lexer of the CSS
// definitions of mdn-data (decoded on first use). The package-level
// functions (Parse, Generate, Walk, ...) behave as its methods do.
func DefaultSyntax() *Syntax {
	return defaultSyntax()
}

// DefaultLexer returns the lexer of the default syntax (upstream lexer).
func DefaultLexer() *Lexer {
	return defaultSyntax().lexer
}

// CreateLexer returns a new lexer for the lexer keys of config with the
// default syntax (see Syntax.CreateLexer).
func CreateLexer(config *Config) (*Lexer, error) {
	return defaultSyntax().CreateLexer(config)
}

// Walk traverses an AST with the default syntax (see Syntax.Walk).
func Walk(root Node, options WalkOptions) error {
	return defaultParseSyntax().Walk(root, options)
}

// Find returns the first node for which fn returns true, or nil, with the
// default syntax (see Syntax.Find).
func Find(ast Node, fn FindFunc) Node {
	return defaultParseSyntax().Find(ast, fn)
}

// FindLast returns the last node for which fn returns true, or nil, with
// the default syntax.
func FindLast(ast Node, fn FindFunc) Node {
	return defaultParseSyntax().FindLast(ast, fn)
}

// FindAll returns every node for which fn returns true, with the default
// syntax.
func FindAll(ast Node, fn FindFunc) []Node {
	return defaultParseSyntax().FindAll(ast, fn)
}

// FromPlainObject builds an AST from its plain object form with the
// default syntax.
func FromPlainObject(ast map[string]any) (Node, error) {
	return defaultParseSyntax().FromPlainObject(ast)
}

// ToPlainObject returns the plain object form of an AST with the default
// syntax.
func ToPlainObject(ast Node) (map[string]any, error) {
	return defaultParseSyntax().ToPlainObject(ast)
}

// Fork returns a new syntax extending the default syntax (see
// Syntax.Fork).
func Fork(extension SyntaxExtension) (*Syntax, error) {
	return defaultSyntax().Fork(extension)
}
