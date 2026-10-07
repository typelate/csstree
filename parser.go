// Ported from lib/parser/create.js, lib/parser/index.js and
// lib/parser/parse-selector.js.

package csstree

import (
	"slices"
	"strings"
	"sync"

	"github.com/typelate/csstree/tokenizer"
)

const (
	exclamationMark  = 0x0021        // U+0021 EXCLAMATION MARK (!)
	numberSign       = 0x0023        // U+0023 NUMBER SIGN (#)
	semicolon        = 0x003B        // U+003B SEMICOLON (;)
	leftCurlyBracket = 0x007B        // U+007B LEFT CURLY BRACKET ({)
	nullTokenType    = tokenizer.EOF // upstream NULL
)

// ParseContext names what a parse parses: the node type of the root of
// the AST (upstream options.context). A syntax may define more contexts in
// Config.Contexts.
type ParseContext string

// The parse contexts of the default syntax.
const (
	ContextDefault         ParseContext = "default" // a StyleSheet, as ContextStylesheet
	ContextStylesheet      ParseContext = "stylesheet"
	ContextAtrule          ParseContext = "atrule"
	ContextAtrulePrelude   ParseContext = "atrulePrelude" // of ParseOptions.Atrule
	ContextMediaQueryList  ParseContext = "mediaQueryList"
	ContextMediaQuery      ParseContext = "mediaQuery"
	ContextCondition       ParseContext = "condition" // of ParseOptions.Kind
	ContextRule            ParseContext = "rule"
	ContextSelectorList    ParseContext = "selectorList"
	ContextSelector        ParseContext = "selector"
	ContextBlock           ParseContext = "block"
	ContextDeclarationList ParseContext = "declarationList"
	ContextDeclaration     ParseContext = "declaration"
	ContextValue           ParseContext = "value"
)

// ParseOptions configures a parse. The zero value means upstream's defaults.
type ParseOptions struct {
	// Context names the parse context; "" means ContextDefault.
	Context ParseContext
	// Atrule is the at-rule name for ContextAtrulePrelude; "" is upstream
	// null.
	Atrule string
	// Kind is the condition kind for ContextCondition ("media",
	// "supports" or "container"); "" means upstream's default "media".
	Kind string

	// Positions makes the parser record node locations.
	Positions bool
	// Filename is used as loc.source; "" means "<unknown>".
	Filename string
	// Offset, Line and Column are the position of the first char of the
	// source; 0 means upstream's defaults (0, 1, 1).
	Offset int
	Line   int
	Column int

	// NoParseAtrulePrelude, NoParseRulePrelude and NoParseValue keep
	// at-rule preludes, rule preludes and declaration values as Raw nodes
	// (upstream parseAtrulePrelude, parseRulePrelude and parseValue set to
	// false; they default to true).
	NoParseAtrulePrelude bool
	NoParseRulePrelude   bool
	NoParseValue         bool
	// ParseCustomProperty parses custom property values as Value nodes
	// instead of Raw nodes.
	ParseCustomProperty bool

	// OnParseError is called when the parser recovers from an error
	// (usually a *SyntaxError) by producing fallbackNode (usually a Raw
	// node). Returning a non-nil error stops the parse with that error, as
	// throwing from upstream's onParseError does.
	OnParseError func(err error, fallbackNode Node) error
	// OnComment is called for every comment with its text (without /* and
	// */) and its location, which is nil unless Positions is set.
	OnComment func(value string, loc *Location)
	// OnToken is called for every token before parsing.
	OnToken func(api *TokenIterateAPI, tokenType tokenizer.TokenType, start, end, index int)
}

// TokenIterateAPI is the `this` of upstream's onToken callback.
type TokenIterateAPI struct {
	Filename   string
	Source     string
	TokenCount int
	Balance    []int

	parser      *Parser
	locationMap *tokenizer.OffsetToLocation
}

// GetTokenType returns the type of the token at index.
func (api *TokenIterateAPI) GetTokenType(index int) tokenizer.TokenType {
	return api.parser.GetTokenType(index)
}

// GetTokenTypeName returns the name of the type of the token at index.
func (api *TokenIterateAPI) GetTokenTypeName(index int) string {
	return api.parser.GetTokenType(index).String()
}

// GetTokenStart returns the start offset of the token at index.
func (api *TokenIterateAPI) GetTokenStart(index int) int {
	return api.parser.GetTokenStart(index)
}

// GetTokenEnd returns the end offset of the token at index.
func (api *TokenIterateAPI) GetTokenEnd(index int) int {
	return api.parser.GetTokenEnd(index)
}

// GetTokenValue returns the text of the token at index.
func (api *TokenIterateAPI) GetTokenValue(index int) string {
	return substring(api.parser.Source, api.parser.GetTokenStart(index), api.parser.GetTokenEnd(index))
}

// Substring returns Source[start:end].
func (api *TokenIterateAPI) Substring(start, end int) string {
	return substring(api.parser.Source, start, end)
}

// IsBlockOpenerTokenType reports whether tokenType opens a block.
func (api *TokenIterateAPI) IsBlockOpenerTokenType(tokenType tokenizer.TokenType) bool {
	return tokenType.IsBlockOpener()
}

// IsBlockCloserTokenType reports whether tokenType closes a block.
func (api *TokenIterateAPI) IsBlockCloserTokenType(tokenType tokenizer.TokenType) bool {
	return tokenType.IsBlockCloser()
}

// GetBlockTokenPairIndex returns the index of the token paired with the
// block token at index, or -1.
func (api *TokenIterateAPI) GetBlockTokenPairIndex(index int) int {
	return api.parser.GetBlockTokenPairIndex(index)
}

// GetLocation returns the position of offset.
func (api *TokenIterateAPI) GetLocation(offset int) tokenizer.SourcePosition {
	return tokenizer.SourcePosition{Filename: api.Filename, Position: api.locationMap.Position(offset)}
}

// GetRangeLocation returns the location of [start, end).
func (api *TokenIterateAPI) GetRangeLocation(start, end int) *Location {
	loc := api.locationMap.Range(start, end)
	return &loc
}

// UnknownContextError is returned by a parse with a context the syntax does
// not define.
type UnknownContextError struct {
	Context ParseContext
}

// Error returns upstream's message, "Unknown context `name`".
func (e *UnknownContextError) Error() string {
	return "Unknown context `" + string(e.Context) + "`"
}

// parseAbort carries an error returned by OnParseError (other than a
// *SyntaxError) up to the parse entry point.
type parseAbort struct {
	err error
}

// Parser is the state of one parse: a token stream plus the syntax
// configuration (upstream builds it with
// Object.assign(new TokenStream(), processConfig(config), { ... })).
//
// Node parse functions are methods named after the node type
// (p.Selector(), p.Block(isStyleBlock), ...). Syntax extensions (parse
// contexts, scopes, at-rule and pseudo parsers, features) receive the
// Parser and call these methods and the helpers below. A Parser is used
// for a single parse and must not be shared between goroutines.
type Parser struct {
	*tokenizer.TokenStream

	// config holds the parser keys of the syntax configuration (upstream
	// this.scope, this.atrule, this.pseudo, this.features), shared by the
	// concurrent parses of the syntax and only read.
	config *Config

	ParseAtrulePrelude  bool
	ParseRulePrelude    bool
	ParseValue          bool
	ParseCustomProperty bool

	filename          string
	needPositions     bool
	onParseError      func(err error, fallbackNode Node) error
	onParseErrorThrow bool
	locationMap       *tokenizer.OffsetToLocation
}

// processConfig returns a deep copy of the parser keys of config (as mix
// copies them: new maps and new recognizers), so that a parse function is
// not affected by later changes to the Config it was built from.
func processConfig(config *Config) *Config {
	parseConfig := mix(nil, &Config{
		Contexts: config.Contexts,
		Features: config.Features,
		Scope:    config.Scope,
		Atrule:   config.Atrule,
		Pseudo:   config.Pseudo,
	})
	if parseConfig.Contexts == nil {
		parseConfig.Contexts = make(map[ParseContext]ContextParser)
	}
	return parseConfig
}

// parseFunc parses source and returns the root node of the context named
// by options.Context.
type parseFunc func(source string, options ParseOptions) (Node, error)

// createParser returns a parse function for config (upstream
// createParser). The function is safe for concurrent use: every call
// uses a new Parser.
func createParser(config *Config) parseFunc {
	if config == nil {
		config = new(Config)
	}
	parseConfig := processConfig(config)

	return func(source string, options ParseOptions) (ast Node, err error) {
		parser := &Parser{
			TokenStream:        new(tokenizer.TokenStream),
			config:             parseConfig,
			ParseAtrulePrelude: true,
			ParseRulePrelude:   true,
			ParseValue:         true,
			locationMap:        new(tokenizer.OffsetToLocation),
		}
		return parser.parse(source, options)
	}
}

func (p *Parser) parse(source string, options ParseOptions) (ast Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case parseAbort:
				ast, err = nil, r.err
			case *SyntaxError:
				ast, err = nil, r
			default:
				// a bug in a parse function, an extension or a callback
				ast, err = nil, newInternalError(r)
			}
		}
	}()

	p.SetSource(source)

	p.filename = options.Filename
	if p.filename == "" {
		p.filename = "<unknown>"
	}
	*p.locationMap = tokenizer.OffsetToLocation{
		Source:      source,
		Filename:    p.filename,
		StartOffset: options.Offset,
		StartLine:   options.Line,
		StartColumn: options.Column,
	}
	p.needPositions = options.Positions
	p.onParseError = options.OnParseError
	p.onParseErrorThrow = false

	p.ParseAtrulePrelude = !options.NoParseAtrulePrelude
	p.ParseRulePrelude = !options.NoParseRulePrelude
	p.ParseValue = !options.NoParseValue
	p.ParseCustomProperty = options.ParseCustomProperty

	context := options.Context
	if context == "" {
		context = ContextDefault
	}

	contextParser, ok := p.config.Contexts[context]
	if !ok || contextParser == nil {
		return nil, &UnknownContextError{Context: context}
	}

	if options.OnToken != nil {
		api := &TokenIterateAPI{
			Filename:    p.filename,
			Source:      source,
			TokenCount:  p.TokenCount,
			Balance:     p.Balance[:p.TokenCount+1],
			parser:      p,
			locationMap: p.locationMap,
		}
		p.ForEachToken(func(tokenType tokenizer.TokenType, start, end, index int) {
			options.OnToken(api, tokenType, start, end, index)
		})
	}

	if options.OnComment != nil {
		p.ForEachToken(func(tokenType tokenizer.TokenType, start, end, _ int) {
			if tokenType == tokenizer.Comment {
				loc := p.Location(start, end)
				var value string
				if tokenizer.CmpStr(source, end-2, end, "*/") {
					value = source[start+2 : end-2]
				} else {
					value = source[start+2 : end]
				}

				options.OnComment(value, loc)
			}
		})
	}

	ast = contextParser(p, options)

	if !p.EOF {
		p.Unexpected()
	}

	return ast, nil
}

// ScopeRecognizer returns the recognizer of the scope name ("AtrulePrelude",
// "Selector", "Value" or one an extension added), or nil, for ReadSequence
// and Function. The recognizer belongs to the syntax and is shared by
// concurrent parses: it must not be changed (change scopes with Fork).
func (p *Parser) ScopeRecognizer(name string) *Recognizer {
	return p.config.Scope[name]
}

// The ConsumeUntil* methods are tokenizer.StopFunc values for Raw and
// SkipUntilBalanced: they get the first char code of a token and say
// whether to stop before it, after it or not at all.

// ConsumeUntilLeftCurlyBracket stops before a {.
func (p *Parser) ConsumeUntilLeftCurlyBracket(code int) tokenizer.StopMode {
	if code == leftCurlyBracket {
		return tokenizer.StopBefore
	}
	return tokenizer.NoStop
}

// ConsumeUntilLeftCurlyBracketOrSemicolon stops before a { or a ;.
func (p *Parser) ConsumeUntilLeftCurlyBracketOrSemicolon(code int) tokenizer.StopMode {
	if code == leftCurlyBracket || code == semicolon {
		return tokenizer.StopBefore
	}
	return tokenizer.NoStop
}

// ConsumeUntilExclamationMarkOrSemicolon stops before a ! or a ;.
func (p *Parser) ConsumeUntilExclamationMarkOrSemicolon(code int) tokenizer.StopMode {
	if code == exclamationMark || code == semicolon {
		return tokenizer.StopBefore
	}
	return tokenizer.NoStop
}

// ConsumeUntilSemicolonIncluded stops after a ;.
func (p *Parser) ConsumeUntilSemicolonIncluded(code int) tokenizer.StopMode {
	if code == semicolon {
		return tokenizer.StopAfter
	}
	return tokenizer.NoStop
}

// scanUntilBalanced returns the index of the first token from start that
// has one of the stop types, skipping blocks, or of the token that ends
// the block start is in (the token count at the end of the source), as
// SkipUntilBalanced would stop, without moving the cursor.
func (p *Parser) scanUntilBalanced(start int, stop ...tokenizer.TokenType) int {
	cursor := start
	for ; cursor < p.TokenCount; cursor++ {
		balanceEnd := p.Balance[cursor]

		// the closing token of the block start is in
		if balanceEnd < start {
			break
		}

		tokenType := p.GetTokenType(cursor)
		if slices.Contains(stop, tokenType) {
			break
		}

		// fast forward to the end of a block
		if tokenType.IsBlockOpener() {
			cursor = balanceEnd
		}
	}
	return min(cursor, p.TokenCount)
}

// ParseWithFallback returns consumer(). When consumer reports a syntax
// error (panics with a *SyntaxError, as Fail does), the parser rewinds to
// where consumer started, returns fallback() instead and reports the error
// to OnParseError. Any other panic is not recovered: it ends the parse
// with an *InternalError.
func (p *Parser) ParseWithFallback(consumer, fallback func() Node) Node {
	startIndex := p.TokenIndex

	node, err := p.try(consumer)
	if err == nil {
		return node
	}

	if p.onParseErrorThrow {
		panic(err)
	}

	p.Skip(startIndex - p.TokenIndex)
	fallbackNode := fallback()

	p.onParseErrorThrow = true
	if p.onParseError != nil {
		if handlerErr := p.onParseError(err, fallbackNode); handlerErr != nil {
			// upstream: the handler throws, which leaves onParseErrorThrow
			// set so every enclosing parseWithFallback rethrows; parseAbort
			// is not recovered by try, so it reaches the parse function
			panic(parseAbort{err: handlerErr})
		}
	}
	p.onParseErrorThrow = false

	return fallbackNode
}

// try calls fn and returns the *SyntaxError it panics with, if any. Any
// other panic goes on: upstream catches any exception, but a Go panic that
// is not a syntax error is a bug, which a Raw fallback would hide (the
// parse function reports it as an *InternalError).
func (p *Parser) try(fn func() Node) (node Node, err *SyntaxError) {
	defer func() {
		if r := recover(); r != nil {
			syntaxErr, ok := r.(*SyntaxError)
			if !ok {
				panic(r)
			}
			node, err = nil, syntaxErr
		}
	}()

	return fn(), nil
}

// LookupNonWSType returns the type of the first token at or after offset
// tokens from the current one that is not whitespace or a comment.
func (p *Parser) LookupNonWSType(offset int) tokenizer.TokenType {
	var tokenType tokenizer.TokenType

	for {
		tokenType = p.LookupType(offset)
		offset++
		if tokenType != tokenizer.WhiteSpace && tokenType != tokenizer.Comment {
			return tokenType
		}
		if tokenType == nullTokenType {
			break
		}
	}

	return nullTokenType
}

// CharCodeAt returns the byte at offset in the source, or 0 out of range.
func (p *Parser) CharCodeAt(offset int) int {
	if offset >= 0 && offset < len(p.Source) {
		return int(p.Source[offset])
	}
	return 0
}

// Substring returns the source between offsetStart and offsetEnd
// (String.prototype.substring semantics).
func (p *Parser) Substring(offsetStart, offsetEnd int) string {
	return substring(p.Source, offsetStart, offsetEnd)
}

// CmpChar reports whether the char at offset, lowercased for A-Z, is
// charCode.
func (p *Parser) CmpChar(offset, charCode int) bool {
	return tokenizer.CmpChar(p.Source, offset, charCode)
}

// CmpStr reports whether the source between offsetStart and offsetEnd,
// lowercased for A-Z, equals str.
func (p *Parser) CmpStr(offsetStart, offsetEnd int, str string) bool {
	return tokenizer.CmpStr(p.Source, offsetStart, offsetEnd, str)
}

// Consume checks that the current token has tokenType, moves past it and
// returns its text.
func (p *Parser) Consume(tokenType tokenizer.TokenType) string {
	start := p.TokenStart

	p.Eat(tokenType)

	return p.SubstrToCursor(start)
}

// ConsumeFunctionName moves past a function token and returns its name
// (without the parenthesis).
func (p *Parser) ConsumeFunctionName() string {
	name := substring(p.Source, p.TokenStart, p.TokenEnd-1)

	p.Eat(tokenizer.Function)

	return name
}

// ConsumeNumber moves past a token of tokenType (a number, percentage or
// dimension) and returns its numeric part.
func (p *Parser) ConsumeNumber(tokenType tokenizer.TokenType) string {
	number := substring(p.Source, p.TokenStart, tokenizer.ConsumeNumber(p.Source, p.TokenStart))

	p.Eat(tokenType)

	return number
}

// Eat moves past the current token, or reports a syntax error when it does
// not have tokenType.
func (p *Parser) Eat(tokenType tokenizer.TokenType) {
	if p.TokenType != tokenType {
		tokenName := tokenType.String()
		tokenName = strings.ReplaceAll(tokenName[:len(tokenName)-6], "-", " ")
		tokenName = strings.ToUpper(tokenName[:1]) + tokenName[1:]
		message := tokenName + " is expected"
		if strings.ContainsAny(tokenName, "[](){}") {
			message = `"` + tokenName + `" is expected`
		}
		offset := p.TokenStart

		// tweak message and offset
		switch tokenType {
		case tokenizer.Ident:
			// when identifier is expected but there is a function or url
			if p.TokenType == tokenizer.Function || p.TokenType == tokenizer.URL {
				offset = p.TokenEnd - 1
				message = "Identifier is expected but function found"
			} else {
				message = "Identifier is expected"
			}

		case tokenizer.Hash:
			if p.IsDelim(numberSign, 0) {
				p.Next()
				offset++
				message = "Name is expected"
			}

		case tokenizer.Percentage:
			if p.TokenType == tokenizer.Number {
				offset = p.TokenEnd
				message = "Percent sign is expected"
			}
		}

		p.FailAt(message, offset)
	}

	p.Next()
}

// EatIdent moves past an identifier token equal to name (ASCII case
// insensitive; name must be lowercase), or reports a syntax error.
func (p *Parser) EatIdent(name string) {
	if p.TokenType != tokenizer.Ident || !p.LookupValue(0, name) {
		p.Fail(`Identifier "` + name + `" is expected`)
	}

	p.Next()
}

// EatDelim moves past a delim token for code, or reports a syntax error.
func (p *Parser) EatDelim(code int) {
	if !p.IsDelim(code, 0) {
		p.Fail(`Delim "` + string(rune(code)) + `" is expected`)
	}

	p.Next()
}

// Location returns the location of [start, end), or nil when positions
// are off.
func (p *Parser) Location(start, end int) *Location {
	if p.needPositions {
		loc := p.locationMap.Range(start, end)
		return &loc
	}

	return nil
}

// LocationFromList returns the location spanning the nodes of list (or
// the current position when it is empty), or nil when positions are off.
func (p *Parser) LocationFromList(list *List[Node]) *Location {
	if p.needPositions {
		var head, tail Node
		if list != nil {
			head, tail = list.First(), list.Last()
		}
		start := p.TokenStart
		if head != nil {
			start = head.Location().Start.Offset - p.locationMap.StartOffset
		}
		end := p.TokenStart
		if tail != nil {
			end = tail.Location().End.Offset - p.locationMap.StartOffset
		}
		loc := p.locationMap.Range(start, end)
		return &loc
	}

	return nil
}

// Unexpected reports an "Unexpected input" syntax error at the current
// token (the end of the source at EOF). Like Fail and FailAt, it panics
// with a *SyntaxError, which ParseWithFallback and the parse function
// recover: extension callbacks may call it anywhere the parser calls them,
// and it does not return.
func (p *Parser) Unexpected() {
	p.fail("Unexpected input", -1)
}

// Fail reports a syntax error with message at the current token (the end
// of the source at EOF). See Unexpected.
func (p *Parser) Fail(message string) {
	p.fail(message, -1)
}

// FailAt reports a syntax error with message at the source offset offset;
// an offset outside the source means the current token, as for Fail. See
// Unexpected.
func (p *Parser) FailAt(message string, offset int) {
	p.fail(message, offset)
}

// fail panics with the *SyntaxError of message at offset (upstream
// error(message, offset), with a negative offset for an omitted one).
func (p *Parser) fail(message string, offset int) {
	source := p.Source
	var location tokenizer.Position
	switch {
	case offset >= 0 && offset < len(source):
		location = p.locationMap.Position(offset)
	case p.EOF:
		location = p.locationMap.Position(tokenizer.FindWhiteSpaceStart(source, len(source)-1))
	default:
		location = p.locationMap.Position(p.TokenStart)
	}

	panic(newSyntaxError(
		message,
		source,
		location.Offset,
		location.Line,
		location.Column,
		max(p.locationMap.StartLine, 1),
		max(p.locationMap.StartColumn, 1),
	))
}

// substring mirrors String.prototype.substring: it clamps both ends to
// [0, len(s)] and swaps them when start > end.
func substring(s string, start, end int) string {
	start = min(max(start, 0), len(s))
	end = min(max(end, 0), len(s))
	if start > end {
		start, end = end, start
	}
	return s[start:end]
}

// selectorParse is upstream lib/parser/parse-selector.js.
var selectorParse = sync.OnceValue(func() parseFunc {
	return createParser(parserSelectorConfig())
})

// Parse parses source with the default syntax (upstream parse).
func Parse(source string, options ParseOptions) (Node, error) {
	return defaultParseSyntax().Parse(source, options)
}

// ParseSelector parses source with the selector-only syntax (upstream
// css-tree/selector-parser): its contexts are "default" and "selectorList"
// (a SelectorList) and "selector" (a Selector).
func ParseSelector(source string, options ParseOptions) (Node, error) {
	return selectorParse()(source, options)
}
