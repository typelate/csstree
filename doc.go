// Go only.

// Package csstree parses CSS into an AST, walks and searches the AST,
// generates CSS from it, and checks CSS values against the grammar of CSS
// properties, types and at-rules. It is a Go port of the JavaScript
// library csstree (https://github.com/csstree/csstree), version 3.2.1
// (see [Version]). Doc comments call that library "upstream".
//
// # Overview
//
// [Parse] turns CSS text into an AST. [Walk] visits its nodes, and [Find],
// [FindLast] and [FindAll] search it. [Generate] turns an AST back into
// CSS text, and [GenerateWithSourceMap] also returns a source map of the
// result. The [Lexer] matches values against the CSS grammar:
// [DefaultLexer] returns the lexer built from the CSS definitions of
// mdn-data, and its Match methods, such as [Lexer.MatchProperty] and
// [Lexer.MatchPropertyString], return a [*MatchResult] and an error.
//
//	ast, err := csstree.Parse(`.a { color: red }`, csstree.ParseOptions{})
//	if err != nil {
//		return err
//	}
//	css, err := csstree.Generate(ast, csstree.GenerateOptions{})
//	if err != nil {
//		return err
//	}
//	if _, err := csstree.DefaultLexer().MatchPropertyString("color", "red"); err != nil {
//		// the value does not match the syntax of color
//	}
//
// The package-level functions use the default syntax. A [Syntax] has the
// same functions as methods; [DefaultSyntax] returns the default syntax,
// and [Fork] and [NewSyntax] create others.
//
// # Parse contexts
//
// [ParseOptions.Context] selects what Parse parses, and so the type of the
// root node it returns. The zero value is [ContextDefault].
//
//	Context                 Root node
//	ContextDefault          *StyleSheet
//	ContextStylesheet       *StyleSheet
//	ContextAtrule           *Atrule
//	ContextAtrulePrelude    *AtrulePrelude (of the at-rule ParseOptions.Atrule)
//	ContextMediaQueryList   *MediaQueryList
//	ContextMediaQuery       *MediaQuery
//	ContextCondition        *Condition (of the kind ParseOptions.Kind)
//	ContextRule             *Rule
//	ContextSelectorList     *SelectorList
//	ContextSelector         *Selector
//	ContextBlock            *Block (of declarations and nested rules)
//	ContextDeclarationList  *DeclarationList
//	ContextDeclaration      *Declaration
//	ContextValue            *Value
//
// A context the syntax does not define fails with an
// [*UnknownContextError]. [ParseSelector] parses with a smaller syntax
// that only has the selector contexts.
//
// # The AST
//
// Each node type is a Go struct, such as [StyleSheet], [Rule] or
// [Declaration], and a pointer to it implements [Node]. Type switches and
// type assertions select node types. Child nodes are held in typed fields
// (Rule.Block is a [*Block]), in [Node] fields when several node types are
// allowed (Rule.Prelude is a [*SelectorList] or a [*Raw]), and in
// [*List] fields for sequences (Children). A nil [*List] reads as an empty
// list. String fields hold "" where upstream has null.
//
// Nodes encode to JSON with the keys and values of upstream's AST, so the
// JSON of an AST matches JSON.stringify of upstream's AST. [ToPlainObject]
// returns that form as maps and slices, and [FromPlainObject] builds an
// AST from it. [Clone] copies an AST.
//
// With [ParseOptions.Positions] set, every node has a [Location].
// Offsets and columns in locations, in [SyntaxError] and
// [SyntaxMatchError], and in source maps count bytes of UTF-8, not UTF-16
// code units as upstream does. Lines and columns start at 1 unless
// [ParseOptions] sets other starting values.
//
// # Extending the syntax
//
// [Fork] (or [Syntax.Fork]) returns a new syntax built from a copy of the
// config of the default syntax (or of the receiver) and an extension.
// The syntax it forks from does not change. The extension is either:
//
//   - a [*Config], whose set fields are merged into the copy: map entries
//     are added or replaced by key, and a Types or Properties syntax that
//     starts with "|" adds alternatives to the existing syntax; or
//   - an [ExtendFunc], which receives the copy, may change it, and returns
//     the config to use.
//
// A config can add or replace:
//
//   - parse contexts ([Config.Contexts]), each a [ContextParser];
//   - scopes ([Config.Scope]): the [Recognizer] that reads the nodes of an
//     at-rule prelude, a selector or a value, and the parsers of functions
//     inside them;
//   - at-rule parsers ([Config.Atrule]), pseudo-class and pseudo-element
//     parsers ([Config.Pseudo]) and the parsers of functional condition
//     features ([Config.Features]);
//   - node types ([Config.Node]): a [NodeConfig] gives a node type its
//     Structure, Generate function and walk context;
//   - the definitions the lexer matches: types, properties, at-rules,
//     units, CSS-wide keywords and generic types.
//
// Parse functions receive the [*Parser] and call its methods: one per
// built-in node type, named after it (p.Selector(), p.Value(), ...), and
// token stream helpers. A fork cannot replace the parse method of a
// built-in node type.
//
// The node types of this package get their Walk, Clone and
// FromPlainObject functions from code generated from their struct tags
// (nodes_gen.go). A node type added by an extension writes these
// functions by hand in its NodeConfig. Without Walk, the walker does not
// descend into the node's children; without Clone, [Syntax.Clone] shares
// the node; without FromPlainObject, [Syntax.FromPlainObject] rejects it.
// To appear in [ToPlainObject] and JSON, the node type needs a MarshalJSON
// method that writes "type". See the Fork example.
//
// # Errors
//
// Functions report failures as error values. Use [errors.Is] and
// [errors.As] to inspect them.
//
//   - Parse returns a [*SyntaxError] for CSS it cannot parse. Its Line,
//     Column and Offset locate the error and Message describes it. Inside
//     style sheets, blocks, rule and at-rule preludes, declaration values
//     and conditions, the parser recovers from syntax errors by keeping
//     the text as a [Raw] node instead; [ParseOptions.OnParseError] sees
//     those errors.
//   - Parse and Generate return an [*InternalError] when a parse or
//     generate function, an extension or a callback panics with anything
//     other than a syntax error. That is a bug; the error holds the panic
//     value and the stack.
//   - Generate returns an [*UnknownNodeTypeError] for a node type without
//     a generate function.
//   - The Match methods of [Lexer] return a [*SyntaxMatchError] for a
//     value that does not match, which unwraps to [ErrIterationLimit] when
//     the matcher gave up; a [*SyntaxReferenceError] for an unknown
//     property, type, at-rule or descriptor; an [*AtruleSyntaxError] for a
//     prelude or descriptor the at-rule does not allow; or one of
//     [ErrVarNotSupported], [ErrCustomProperty] and [ErrNotDeclaration].
//     [Lexer] lists the rest.
//   - Walk returns [ErrNoWalkHandler] or an [*UnknownVisitError] for bad
//     [WalkOptions].
//
// # Concurrency
//
// A [Syntax] and a [Lexer] do not change once created and are safe for
// concurrent use, and so are [DefaultSyntax], [DefaultLexer] and the
// package-level functions.
//
// Several goroutines may read one AST at the same time: [Generate],
// [GenerateWithSourceMap], [Find], [FindLast], [FindAll], [Clone],
// [ToPlainObject] and the lexer's Match, Find*Fragments and
// CheckStructure methods only read it. [Walk] needs exclusive access to
// the AST it walks, even when its handlers do not change it: it records
// its position in each list it walks, so that handlers may insert and
// remove items. Any change to an AST also needs exclusive access.
//
// A [List] may be read with All and Backward by several goroutines while
// nobody changes it; its other methods, including ForEach, need
// exclusive access. A [Parser] belongs to a single parse. A
// [SourceMapGenerator] is not safe for concurrent use, not even for
// String, which may sort its mappings.
package csstree
