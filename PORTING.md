# Porting notes

This module is a file-by-file Go port of [csstree](https://github.com/csstree/csstree)
v3.2.1 (commit `f8980154f79110833bf56c391cd45fe5f9205752`). Upstream is MIT
licensed; see `LICENSE`. The CSS syntax data in `data/mdn` comes from
[mdn-data](https://github.com/mdn/data) 2.27.1 (CC0, see `data/mdn/LICENSE`).

A pristine upstream checkout lives at `~/Code/tmp/csstree-upstream` for reference.

## Method

1. Port the upstream test file first (`lib/__tests/*.js` → `*_test.go`), keeping
   `describe`/`it` names as nested `t.Run` names so failures map back to upstream.
2. Port each source file line by line into the Go file named in the map below.
   Keep upstream function names (Go-cased), ordering, comments and control flow so
   the two can be read side by side. Each Go file starts with a comment naming the
   upstream file it was ported from, e.g. `// Ported from lib/tokenizer/TokenStream.js.`
3. Fixtures are shared with upstream unchanged: `fixtures/` is copied to `testdata/`.
4. Where JavaScript semantics cannot be carried over, adapt the smallest amount
   and record it under "Deviations" below. Never silently drop a test: either port
   it, or keep it as a `t.Skip("not ported: <reason>")` subtest.

## Packages

| Go package | Upstream |
|---|---|
| `github.com/typelate/csstree/tokenizer` | `lib/tokenizer/*` |
| `github.com/typelate/csstree/definitionsyntax` | `lib/definition-syntax/*` |
| `github.com/typelate/csstree` (root) | everything else: `lib/utils`, `lib/parser`, `lib/syntax`, `lib/generator`, `lib/walker`, `lib/convertor`, `lib/lexer`, `lib/data*.js`, `lib/index.js` |

The root package is large on purpose: AST types, parser, generator, walker and
lexer are mutually dependent, and splitting them would need interfaces whose
only purpose is breaking import cycles.

## File map

Upstream path → Go file (root package unless stated). Use snake_case of the
upstream basename, prefixed by its directory when needed to avoid collisions.

- `lib/tokenizer/X.js` → `tokenizer/x.go` (`TokenStream.js` → `token_stream.go`,
  `char-code-definitions.js` → `char_code_definitions.go`, `index.js` → `tokenizer.go`)
- `lib/definition-syntax/X.js` → `definitionsyntax/x.go`
- `lib/utils/List.js` → `list.go`; `lib/utils/names.js` → `names.go`;
  `lib/utils/string.js` → `string.go`; `ident.js` → `ident.go`; `url.js` → `url.go`;
  `clone.js` → `clone.go`
- `lib/parser/create.js` → `parser.go`; `lib/parser/sequence.js` → `parser_sequence.go`;
  `lib/parser/SyntaxError.js` → `parser_syntax_error.go`
- `lib/syntax/node/X.js` → `node_x.go` (`AtrulePrelude.js` → `node_atrule_prelude.go`)
- `lib/syntax/scope/X.js` → `scope_x.go`; `lib/syntax/atrule/X.js` → `atrule_x.go`;
  `lib/syntax/pseudo/X.js` → `pseudo_x.go`; `lib/syntax/function/X.js` → `function_x.go`;
  `lib/syntax/config/X.js` → `config_x.go`; `lib/syntax/create.js` → `syntax.go`
- `lib/generator/X.js` → `generator.go`, `generator_source_map.go`, `generator_token_before.go`
- `lib/walker/create.js` → `walker.go`; `lib/convertor/create.js` → `convertor.go`
- `lib/lexer/X.js` → `lexer_x.go` (`Lexer.js` → `lexer.go`, `match-graph.js` → `lexer_match_graph.go`)
- `lib/data.js` → `data.go` (embeds `data/`)
- `lib/__tests/X.js` → `X_test.go` in the package under test (`tokenizer.js` → `tokenizer/tokenizer_test.go`)

## Go conventions for this port

- Names: JS `camelCase` → Go `CamelCase` when exported upstream (part of the
  public API: exported from `lib/index.js` or a package entry point, or used by
  syntax extensions through `this.` in a parser/generator/lexer callback), else
  `camelCase`. Initialisms stay all caps (`URL`, `ID`).
- Strings are Go strings indexed by **byte**. `source.charCodeAt(i)` becomes a byte
  read (returning 0 past the end, as upstream's helpers do). Every byte >= 0x80
  belongs to a non-ASCII code point, which the tokenizer already classifies as a
  name character, so tokenization agrees with upstream. Offsets and columns are
  byte counts (like `go/token`), not UTF-16 code units.
- Typed arrays (`Uint32Array`, ...) become `[]uint32`/`[]int`.
- Errors: upstream throws. Inside a package, a port may `panic` with a typed error
  value and `recover` it where upstream uses `try`/`catch` (as `text/template` and
  `encoding/json` do), but no panic may escape an exported function: exported
  entry points return `error`.
- `null`/`undefined` → `nil` for pointers/interfaces/slices, or an explicit
  zero value documented at the field.
- `Object.create(null)` dictionaries → `map[string]T`. Where upstream depends on
  insertion order of object keys, keep a slice alongside the map.
- No `reflect` in non-test code. No package-level mutable state except lazily
  computed read-only tables (`sync.OnceValue`).
- Tests: standard `testing` only (no testify). Table-driven where upstream loops
  over fixtures. Failure messages read `Parse(%q) = %v, want %v`.

## AST

- Every node is a pointer to a struct implementing `Node`:

  ```go
  type Node interface {
      Type() string        // upstream node.type, e.g. "Selector"
      Location() *Location // upstream node.loc, nil when positions are off
  }
  ```

  Each struct has `Loc *Location` and the upstream fields (Go-cased) in upstream
  order. Fields that hold a node are `Node` (or a concrete pointer when upstream
  structure allows exactly one node type and null). Children are `*List[Node]`.
- JSON encoding of a node matches upstream `JSON.stringify(ast)`: `"type"` first,
  then `"loc"`, then fields under their upstream names, `null` where upstream has
  null, lists as arrays. AST fixture tests compare by decoding both sides to `any`.
- `Location` mirrors upstream `loc`: `{source, start: {offset, line, column}, end: {...}}`.

## Deviations

Record every intentional difference from upstream here (file, what, why).
- list: `List` is generic (`List[T]`, `ListItem[T]` with exported `Prev`, `Next`, `Data`); `head`/`tail` are unexported fields read through `Head()`/`Tail()` so callers cannot break the links. `size`, `isEmpty`, `first`, `last` are methods; `First`/`Last` return the zero value of `T` (nil for `List[Node]`) where upstream returns null.
- list: the static `List.createItem` is `NewListItem[T]`; type inference takes `T` from the argument, so use `list.CreateItem(data)` (or `NewListItem[Node]`) for a `List[Node]`.
- list: `reduce`/`reduceRight` are package functions `Reduce`/`ReduceRight` because Go methods cannot add the accumulator type parameter. `Map` maps `T` to `T` only (upstream only maps nodes to nodes, in `clone`).
- list: callbacks take `(data, item, list)` without upstream's `thisArg`; the "iterate with thisArg" tests are skipped. `[Symbol.iterator]` is `All() iter.Seq[T]`; `toJSON` is `MarshalJSON`, which always produces an array (`[]` when empty).
- list: `replace(oldItem, itemOrList)` is split into `Replace(oldItem, newItem)` and `ReplaceList(oldItem, list)`. Optional `before` arguments of `insert`, `insertData`, `insertList` are explicit (pass nil to append).
- list: upstream pools released cursors in a module-level free list; the port keeps a free list per `List` to avoid package-level mutable state. A `List` is therefore not safe for concurrent use even by readers that only iterate (as upstream, iteration writes cursor state).
- list: `insert` calls `updateCursors(null, item)` with nextOld/nextNew undefined, which matches no cursor; the port passes nil for both, which only ever sets a nil `next` to nil, so it is equally a no-op on `next`.
- list: misuse that upstream throws for (`before`/`item` not in the list) panics with an unexported error value in `Insert`/`InsertData`/`Remove`. These are programming errors like an out-of-range index and upstream never catches them, so returning `error` would burden every call site with an impossible failure.
- list: the tests store strings rather than distinct empty objects. Two upstream assertions (`#prependList()`, `#insertList()`) expect data `bar` where the list holds `foo` and pass only because `{}` deep-equals `{}`; the port asserts `foo`. `TestListMutateAhead` is Go-only: upstream's mutation tests only touch the current item, which never exercises `updateCursors`.
- names: no cache. Upstream memoizes descriptors in two unbounded module-level Maps keyed by every input; the port recomputes (one `strings.ToLower` and a few slices) and returns descriptor values, so "result should be immutable" holds by copy and "should return the same object" compares values instead of identity.
- names: optional `offset` of `vendorPrefix`/`isCustomProperty` is explicit (`VendorPrefix(s, 0)`). Descriptor types are `KeywordDescriptor`/`PropertyDescriptor` (the functions take the names `Keyword`/`Property`), with JSON tags matching upstream keys. Lower-casing uses `strings.ToLower`, which differs from JS `toLowerCase` for a few special-cased code points (e.g. U+0130).
- tokenizer: `isBOM` → `IsBOM(code int) int` takes a decoded code point (callers use
  `utf8.DecodeRuneInString`) and returns 3, the UTF-8 byte length of U+FEFF/U+FFFE,
  where upstream returns 1 UTF-16 code unit. `Tokenize` and `OffsetToLocation` skip 3 bytes.
- tokenizer: token types are `type TokenType int` constants; upstream `Url`/`BadUrl` are
  `URL`/`BadURL`, `EofCategory` is `EOFCategory`, `isNonAscii` is `IsNonASCII`,
  `consumeBadUrlRemnants` is `ConsumeBadURLRemnants`. `TokenType` has a `String()` method
  returning the `TokenNames` entry (Go only). `tokenTypes` (the namespace object) has no Go
  counterpart: the constants live in package `tokenizer`.
- tokenizer: `CharCodeAt(s, i)` is a Go-only export: `s.charCodeAt(i)` returning 0 out of
  range where JS returns NaN. Every place upstream reads past the end compares the result
  against a non-zero code, so 0 and NaN behave the same.
- tokenizer: `TokenStream` keeps `offsetAndType` and `firstCharOffset` unexported (nothing
  outside lib/tokenizer reads them); `Balance` is exported (`[]int`) for the parser's token
  iterate API. Reads of `offsetAndType[index - 1]` that upstream lets go out of range
  (`lookupOffset`, `lookupValue`, `skip` with index 0) return 0, as a JS typed array read of
  `undefined` does once masked. `isDelim(code, offset)` → `IsDelim(code, offset int)`; pass 0
  for upstream's omitted offset. A nil `tokenize` func means upstream's no-op default.
- tokenizer: `TokenStream.Dump()` returns `[]DumpToken` (JSON tags idx/type/chunk/balance).
- tokenizer: `OffsetToLocation.GetLocation` returns `SourcePosition{Source, Position}` and
  `GetLocationRange` returns `*Location`; `Position`/`Location` carry upstream's JSON keys.
  `NewOffsetToLocation(source, startOffset, startLine, startColumn)` has no defaults: callers
  pass upstream's defaults (0, 1, 1) themselves.
- tokenizer: `computeLinesAndColumns` adopts `len(source)+1` cells instead of `len(source)`:
  upstream writes cell `len(source)` past a reused typed array of exactly that length, which
  JS silently drops and Go would panic on.
- tokenizer: `DecodeEscaped` reads the leading hex digits like `parseInt(s, 16)`; when there
  are none (upstream: `NaN`, then `String.fromCodePoint` throws) it returns U+FFFD.
  A single non-hex escaped char is one byte; the caller appends the rest of a multi-byte
  code point as upstream appends the second half of a surrogate pair.
- tokenizer: tests `should convert input to string` and `should accept a Buffer` are skipped:
  Go sources are typed `string`. Fixture test names omit JsonLocator's `line:column` prefix.
  No fixture contains non-ASCII text, so no expected offsets needed converting.
- definitionsyntax: `Node` is sealed (unexported method), so a node of an unknown
  type cannot exist; the "should throw an exception on bad node type" generate test
  is skipped. `Generate` returns a string with no error: a nil node (or a lone
  `*Range`) generates `""` where upstream throws `Unknown node type`. `Walk` returns
  an error for a nil node (`Unknown type: <nil>`) and when neither handler is set.
- definitionsyntax: `walk(node, fn | {enter, leave}, context)` → `Walk(node, WalkOptions{Enter, Leave})`;
  a lone function is `Enter`, and `context` is dropped (closures). `generate(node, fn | options)`
  → `Generate(node, GenerateOptions{ForceBraces, Compact, Decorate})`; a lone function is `Decorate`.
- definitionsyntax: `Range` min/max are `*RangeBound{Value float64, Unit string}`; nil is upstream
  `null` (infinite). Upstream holds a number, or a string like `"0s"`/`"-90deg"` when a unit
  follows; `Unit != ""` is that string case (the lexer skips range checks for it).
  `RangeBound.String()`/JSON reproduce upstream text (`-0` → `0`).
- definitionsyntax: `Multiplier.Min`/`Max` are `int`; a multiplier number too large for an int is a
  `SyntaxError` "Number is out of range" (upstream keeps an inexact float).
- definitionsyntax: the scanner reads a Token as one UTF-8 code point (upstream: one UTF-16 code
  unit; differs only for characters outside the BMP), and `∞` in a range is matched as a 3-byte
  prefix. `SyntaxError.Offset` and the caret line count bytes.
- definitionsyntax: `SyntaxError.Error()` is upstream `error.message` (no `SyntaxError: ` prefix;
  use `errors.As`). Error texts keep upstream capitalization so tests match. No `stack`/`name`.
- definitionsyntax: the fixture loader ports only the `tests` export (with JsonLocator names
  `testdata/definition-syntax/<file>:<line>:<col> (<key>)`); `forEachTest`, `atruleTests`,
  `forEachAtrulePreludeTest` and `forEachAtruleDescriptorTest` need a lexer and belong to the
  root package.
- tokenizer: tokens are packed as `type << 58 | end` in `uint64` cells instead of upstream's `type << 24 | end` in `uint32`, which silently corrupts offsets for sources of 16 MiB or more. `ConsumeName` and `ConsumeBadURLRemnants` read through the bounds-checked `getCharCode`, so an out-of-range offset behaves like upstream's `charCodeAt` instead of panicking.
- utils: `string.decode`/`encode`, `ident.decode`/`encode`, `url.decode`/`encode` are `DecodeString`/`EncodeString(str, apostrophe bool)`, `DecodeIdent`/`EncodeIdent`, `DecodeURL`/`EncodeURL`. They work on bytes: every byte >= 0x80 passes through unescaped (upstream: every code unit >= U+0080), so UTF-8 text round-trips. The decode-encode tests write upstream's U+0080..U+00FF inputs as UTF-8 and its lone surrogates as WTF-8 bytes. `create-custom-error.js` has no counterpart: `SyntaxError` is a Go type (no `name`/`stack`).
- node: every node type is a struct (`*Selector`, ...) with `Loc *Location` and its upstream fields; `Type()` returns the upstream name, so `IdSelector` is `IDSelector` and `Url` is `URL` in Go but `"IdSelector"`/`"Url"` in `Type()` and JSON. Each implements `MarshalJSON` with `"type"`, `"loc"`, then fields in structure order; JSON is written without HTML escaping, like `JSON.stringify`.
- node: `[String, null]` fields (`AnPlusB.a`/`b`, `AttributeSelector.matcher`/`flags`, `MediaQuery.modifier`/`mediaType`, `FeatureRange.rightComparison`, `GeneralEnclosed.function`) are `string` with `""` meaning null (the parser never produces an empty string there); JSON writes `null`. `Declaration.important` (`[Boolean, String]`) is a string: `""` is `false`, `"important"` is `true`, anything else is upstream's raw string (`!IMPORTANT`, `!ie`).
- node: upstream `structure` is `NodeConfig.Structure []StructureField{Name, Types []FieldType{Kind, Node, List}}` in upstream key order (`FieldString`, `FieldBoolean`, `FieldNull`, `FieldNode`, `FieldList`); CDC/CDO's `structure = []` and NestingSelector's `{}` are empty slices. `nodeConfigs()` replaces `lib/syntax/node/index.js`; `index-parse.js`/`index-parse-selector.js`/`index-generate.js` have no counterparts (see parser: node parse methods).
- node: `WhiteSpace` parse returns a new node instead of upstream's frozen shared `SPACE`. Raw's `this.firstCharOffset` is read as `GetTokenStart(0)` (the field is unexported in `tokenizer`).
- node: `Ratio` generate with a nil `Right` fails with `Unknown node type: undefined`, as upstream's `this.node(NumberToken, 1)` does.
- node: `Condition`'s `parentheses[kind]` is a switch; an unknown kind reports a `SyntaxError` ("Unknown condition kind `x`") inside the same `parseWithFallback` where upstream gets a TypeError. `Condition("")`, `FeatureFunction("")`, `FeatureRange("")` use upstream's default parameters (`media`, `unknown`, `unknown`).
- node: `GeneralEnclosed` passes upstream a List through `parseWithFallback`; the port assigns the list from the closures and reports the Raw node (not the list) as the fallback node to `OnParseError`.
- parser: `Parser` embeds `*tokenizer.TokenStream`; node parse functions are exported `*Parser` methods named after the node (`p.Selector()`, `p.Block(isStyleBlock)`, `p.Raw(consumeUntil, excludeWhiteSpace)`, `p.Function(readSequence, recognizer)`, `p.AtrulePrelude(name)`, `p.Condition(kind)`, `p.IDSelector()`, `p.URL()`, ...). They are not overridable from a `Config` (upstream lets `config.node[name].parse` replace them); the configurable extension points are `ParseContext`, `Scope`, `Atrule`, `Pseudo`, `Features` and node `Generate`. Because of this the selector parser (`ParseSelector`) has every node method available, not only the `index-parse-selector.js` subset.
- parser: upstream `this.scope`/`this.atrule`/`this.pseudo`/`this.features` are `p.Config.Scope`/`.Atrule`/`.Pseudo`/`.Features` (a field `Scope` would clash with the `Scope()` node method). A scope object is a `*Recognizer{GetNode, OnWhiteSpace, Functions}`; its function-name keys (`var`, `expression`, import's `layer`/`supports`) live in `Functions`. The readSequence context is `*SequenceContext{Recognizer}`. `atrule[name].parse` is `AtruleParser{Prelude, Block}` (nil = null) and `pseudo[name].parse` is a func.
- parser: parse contexts must be funcs (`func(p *Parser, options ParseOptions) Node`); upstream's string form (`'StyleSheet'` → `this.StyleSheet()`) would need reflection.
- parser: `list: false` (arrays instead of List) is not ported: children are always `*List[Node]`. `createList` etc. are methods returning `*List[Node]`.
- parser: `ParseOptions`'s zero value is upstream's defaults: `parseAtrulePrelude`/`parseRulePrelude`/`parseValue` (default true) are `NoParseAtrulePrelude`/`NoParseRulePrelude`/`NoParseValue`; `Context ""` is `default`; `Filename ""` is `<unknown>`; `Line`/`Column` 0 mean 1 (so line or column 0 cannot be requested); `Atrule ""` is null; `Kind ""` is `media`.
- parser: `OnParseError` returns an error: non-nil stops the parse and `Parse` returns it, as throwing from upstream's handler does (the fixture tests return the `*SyntaxError` for `throw error`). It runs inside the parse, so it is called with `onParseErrorThrow` semantics kept: nested `ParseWithFallback`s let the error through. `OnToken` takes only the function form, as `func(api *TokenIterateAPI, type, start, end, index)` where `api` is upstream's `this`; the array form is not ported.
- parser: internally a syntax error panics with `*SyntaxError`; `ParseWithFallback` recovers only `*SyntaxError` (upstream's catch takes any exception) and re-panics anything else; the parse function recovers `*SyntaxError` and handler errors and returns them. `Error(message, offset)` takes `offset < 0` for upstream's omitted offset and `""` for the default message.
- parser: `createParser` → `CreateParser(*Config) ParseFunc`; each call of the returned func uses a new `Parser` (upstream reuses one), so parse functions are safe for concurrent use. `Parse` and `ParseSelector` (upstream `css-tree/selector-parser`) use parsers built once with `sync.OnceValue`. An unknown context returns `*UnknownContextError` ("Unknown context `x`"). `parse.SyntaxError` and `parse.config` are not exposed.
- parser: `SyntaxError.sourceFragment(extraLines)` is `SourceFragment(extraLines int)` (pass 0 for upstream's default) and `formattedMessage` is `FormattedMessage()`. Line lengths and the 100-column cut count bytes.
- config: `Config` (config.go) types upstream's config object: parser keys, `Node map[string]NodeConfig{Name, Structure, WalkContext, Generate, Walk, Clone, FromPlainObject}` and lexer keys carried as `Generic *bool` (nil = absent, so a fork can tell `generic: false` from no `generic`), `CSSWideKeywords`, `Units`, `Types`, `Atrules map[string]AtruleDefinition`, `Properties`. Configs are built by funcs (`parserConfig()`, `parserSelectorConfig()`, `generatorConfig()`, `walkerConfig()`, `lexerConfig()`, which returns an error only if the embedded data is malformed). `generatorConfig` is upstream's `css-tree/generator` entry point; `Generate` uses the default syntax, so only the exports test uses it.
- generator: `createGenerator` → `CreateGenerator(*Config)`, `generate` → `Generate(node, GenerateOptions{Mode, Decorator}) (string, error)`; an unknown node type is `*UnknownNodeTypeError`. Handlers are `GenerateHandlers{Node, TokenBefore, Token, Emit, Result}` whose funcs take the handlers in use as `h` (upstream `this`); `Decorator` gets and returns a copy. A node generate func is `func(g *Generator, node Node)`; `this.children(node, delimiter)` is `g.Children(node.Children, delimiter)` (nil delimiter = none) because Go nodes have no common children accessor. Source maps: see sourcemap below.
- generator: token-before pair tables hold precomputed codes (`tokenPairCode(type)`, `charPairCode(char)`) instead of mixing token types and chars; the `spec`/`safe` maps are built once with `sync.OnceValue`.
- tests: parse/generate fixture names are `<dir>/<file> <key>[#n]`; error positions are computed from the `^` marker as upstream does (`\r\n` counts as two breaks). Walk tests compare with an `OrderedAST` decoded from the fixture with keys in source order (upstream's `Object.values` order). common.js `JSON.stringify()` is ported (compares the JSON of `stringify.css` to `stringify.ast` exactly).
- tokenizer: `ConsumeBadURLRemnants` clamps its result to `len(source)`. Upstream returns `len+1` when the source ends in an escape inside a bad URL (e.g. `url((\0`) and relies on JavaScript slicing to clamp it; in Go the generator's slice would panic. Found by `FuzzTokenize`/`FuzzParse`.
- walker: `createWalker(config)` → `CreateWalker(*Config) (*Walker, error)` with methods `Walk(root, WalkOptions{Enter, Leave, Reverse, Visit}) error`, `Find`, `FindLast`, `FindAll`. A lone function (`walk(ast, fn)`) is `WalkOptions{Enter: fn}`. Handlers are `func(context *WalkContext, node, item, list) WalkAction`: upstream `this` is the first argument, a `*WalkContext{Root, Stylesheet, Atrule, AtrulePrelude, Rule, Selector, Block, Declaration, Function}` of `Node`s (other walk context names are ignored); `this.break`/`walk.break` and `this.skip`/`walk.skip` are the return values `WalkBreak`/`WalkSkip` (`WalkContinue` is upstream's undefined). Find callbacks return bool. Errors (no handler, bad `visit`, a node type without `structure`) are returned with upstream's messages.
- walker: upstream iterates the fields named by `structure` with `node[field.name]`; a typed AST can't, so each node type has `NodeConfig.Walk` (`WalkFieldsFunc(node, reverse, walkNode, walkList) bool`) visiting its node/list fields in structure order or reversed. Structure still decides whether a type has an iterator (no node/list field: none); a type with such fields but no `Walk` (e.g. a fork's `{structure}`-only node type) is not descended into. Nil fields are skipped even when structure does not allow null (upstream would throw), and nil nodes (root or list data) are not visited.
- convertor: `createConvertor(walk)` → `CreateConvertor(*Config) *Convertor` (it needs the node types' `FromPlainObject`, not the walker). Upstream converts in place between List and array children; Go children are always `*List[Node]`, so the plain object is a separate value: `ToPlainObject(node) (map[string]any, error)` is `JSON.parse(JSON.stringify(ast))` (encoding/json round trip, so a fork's node type needs a `MarshalJSON` writing `type`), and `FromPlainObject(map[string]any) (Node, error)` builds typed nodes with each type's `NodeConfig.FromPlainObject(r *PlainObjectReader, object)`. `PlainObjectReader` reads fields (`Location`, `String`, `Node`, `List`, generic `PlainObjectNode[T]`, `Fail`) and keeps the first error; an unknown node type (`*UnknownNodeTypeError`), a missing `type` or a field of the wrong kind fails where upstream leaves the object as is. This makes JSON → AST possible: decode into `map[string]any`, then `FromPlainObject`. Generate's `(plain object)` tests run `Generate(FromPlainObject(ToPlainObject(ast)))`.
- clone: `clone(node)` → `Clone(node Node) Node` using each node type's `NodeConfig.Clone` (`CloneFunc(node, clone)`): a shallow struct copy with `Loc` copied and node/list fields cloned. A node of a type without a Clone function is shared, not copied (upstream copies any object); `Syntax.Clone` (Go only) also clones a fork's node types. The "an AST as JSON" clone test clones `FromPlainObject(ToPlainObject(ast))`.
- syntax: `createSyntax(config)` → `CreateSyntax(*Config) (*Syntax, error)`; `Syntax` has `Parse`, `Generate`, `Walk`, `Find`, `FindLast`, `FindAll`, `ToPlainObject`, `FromPlainObject`, `Tokenize`, `Fork`, `Clone` (Go only), `CreateLexer` and the field `Lexer *Lexer` (see lexer below). `fork(extension)` → `Fork(SyntaxExtension) (*Syntax, error)`; a `SyntaxExtension` is a `*Config` (mixed in) or an `ExtendFunc(func(base *Config) *Config)` (upstream's object and function forms); nil forks an unchanged copy. The default syntax (`lib/syntax/index.js`) is built once with `sync.OnceValue` and returned by `DefaultSyntax()`; `Fork`, `DefaultLexer` and `CreateLexer` delegate to it. `Parse`, `Generate`, `Walk`, `Find`, `FindLast`, `FindAll`, `ToPlainObject`, `FromPlainObject`, `Tokenize` delegate to a second syntax built the same way without the data definitions (types, properties, at-rules), so a program that only parses never decodes mdn-data; the two parse, generate and walk identically (`ParseSelector` keeps its own parser). Building either panics only if a built-in node type lacked a valid structure or the embedded data were malformed, which tests rule out. `version` is the constant `Version`.
- syntax: `mix(dest, src)` treats a zero field of `src` (nil map/slice/func, nil `Generic`, `""`) as an absent key. `appendOrSet` treats `""` as null and matches `\s` as ASCII whitespace only. Scope recognizers are merged field by field (`GetNode`, `OnWhiteSpace` when set; `Functions` by name) into new `*Recognizer`s, so a fork's function extension may mutate `base.Scope[name]` without affecting the base syntax. `atrule`/`pseudo` entries replace (upstream copies only `parse`, which is the whole Go value). Node configs are merged field by field (`Name`, `Structure`, `WalkContext`, `Generate`, and the Go-only `Walk`, `Clone`, `FromPlainObject`). An `atrules` entry whose descriptors end up empty gets nil descriptors even where upstream's `if (!atrule) continue` would leave `{}` (a Go map value cannot be null).
- sourcemap: `generate(ast, {sourceMap: true})` returning `{css, map}` → `GenerateOptions.SourceMap *SourceMapGenerator`: pass a new generator and Generate fills it while returning the CSS. `SourceMapGenerator` is the part of source-map-js's generator that is used: `AddMapping(SourceMapping{Generated, Original, Source, Name}) error` (with upstream's validation message; `""`/nil are null), `SetSourceContent`, `File`, `String()` (upstream `toString`, same key order, JSON.stringify escaping) and `MarshalJSON`; no `sourceRoot`, `applySourceMap` or consumers. Generated columns count bytes, not UTF-16 code units. An invalid mapping (e.g. a node whose `Loc.Source` is `""`) makes Generate fail. `testdata/sourceMaps/autoprefixer.css` is not used by any upstream test.
- lexer: `new Lexer(config, syntax, structure)` → `NewLexer(config *Config, syntax *Syntax) (*Lexer, error)` (a nil syntax means the default one; the structure argument is internal, used by `Syntax.CreateLexer`). `createSyntax` builds `Syntax.Lexer` as upstream does; `syntax.createLexer(config)` → `Syntax.CreateLexer(*Config) (*Lexer, error)`; upstream's `lexer` export is `DefaultLexer()` and `createLexer` is `CreateLexer`. Construction fails (upstream throws) on a missing or bad node `Structure` and on CSS-wide keywords that do not parse.
- lexer: values to match are a `Node` (nil = upstream null/undefined) or, through the `...String` methods (`MatchString`, `MatchTypeString`, `MatchPropertyString`, `MatchAtrulePreludeString`, `MatchAtruleDescriptorString`, `MatchDescriptorString`, `CheckAtrulePreludeString`), CSS text that is tokenized, not parsed (upstream takes either in one argument). `match(syntax, value)`: a string syntax is `Match`/`MatchString`; a descriptor (e.g. from `GetType`) is `MatchDescriptor`/`MatchDescriptorString` (nil is upstream's `{}`: "Bad syntax"); a definition syntax tree goes through `CreateDescriptorFromTree` first (`CreateDescriptor` takes text).
- lexer: match results are `*MatchResult{Matched *MatchNode, Iterations int, Error error}` with methods `GetTrace`, `IsType`, `IsProperty`, `IsKeyword`. `MatchNode{Syntax MatchSyntax, Match []*MatchNode, Token string, Node Node}`: a syntax node has a non-nil (possibly empty) `Match`, a token leaf a nil one (upstream tells them apart by `Array.isArray(match)` / `'node' in`). `MatchSyntax` is `interface{ Type() string }`: a `definitionsyntax.Node`, or a `*SyntaxRef{Kind, Name}` for upstream's descriptor ref `{type, name}` at the root of a match. `GetTrace` returns nil for upstream null and an empty slice for `[]`.
- lexer: everything upstream throws while matching (a reference to a missing type/property "Bad syntax reference: ...", a definition syntax that does not parse when first used, an AST node the generator does not know, `buildMatchGraph` on a null syntax) is returned as `MatchResult.Error` (`Iterations` 0) instead of escaping; `Validate` and `Dump` return such errors. Upstream's plain `Error`s are the sentinels `ErrVarNotSupported`, `ErrCustomProperty`, `ErrNotDeclaration`; its built-in `SyntaxError`s (at-rule prelude/descriptor presence) are `*AtruleSyntaxError`; `SyntaxReferenceError` and `SyntaxMatchError` are Go types with upstream's fields (`Reference`; `RawMessage`, `Syntax`, `CSS`, `MismatchOffset`, `MismatchLength`, `Offset`, `Line`, `Column`, `Loc`) and messages; `Error()` is upstream `message`. Offsets, columns and the caret line count bytes.
- lexer: descriptors (`*Descriptor{Type, Name, Parent, Serializable}`) parse their syntax and build their match graphs lazily with `sync.OnceValues` (upstream redefines getters), so the shared default lexer is safe for concurrent matching; `Descriptor.Syntax() (definitionsyntax.Node, error)` is upstream `descriptor.syntax` (nil for generic types); match graphs are unexported. `lexer.types`/`properties`/`atrules` are `Types()`/`Properties()`/`Atrules()` iterators in insertion order plus `GetType`/`GetProperty`/`GetAtrule`; `units`, `cssWideKeywords`, `generic` are `Units()` (copy), `CSSWideKeywords()` (copy), `Generic()`. The `Add*_` methods are not safe for concurrent use. An at-rule is `*AtruleSyntax{Name, Prelude, Descriptors}`.
- lexer: Go maps have no insertion order, so `NewLexer` adds `Config.Types`, `Atrules`, `Properties` (and at-rule descriptors) in sorted key order; generic types follow the config types in upstream's order. `Validate` therefore lists errors in that order (upstream: object key order). `GetAtrule`/`GetAtrulePrelude`/`GetProperty` take upstream's optional `fallbackBasename` explicitly (pass true for the default).
- lexer: `getAtruleDescriptor` reads `this.atrules.declarators`, a field that never exists, so upstream always returns null; `GetAtruleDescriptor` returns nil. `matchAtrule` does not exist upstream and is not added.
- lexer: `dump(syntaxAsAst, pretty)` → `Dump(pretty bool) (*LexerDump, error)` with string syntaxes only (the AST form is `Descriptor.Syntax()`); `toString` is `String()` (JSON with Go's sorted map keys, upstream: insertion order). `validate()` → `Validate() (*LexerValidation, error)` (nil = upstream null).
- lexer: `checkStructure(ast)` reads node fields by name; the typed AST cannot, so `CheckStructure(ast Node) ([]StructureWarning, error)` checks `ToPlainObject(ast)` with the generic checker of structure.js (`NodeConfig.Structure`), and `CheckPlainObjectStructure(map[string]any) []StructureWarning` checks a plain object (e.g. decoded JSON) directly. Warnings hold the offending node's plain object (`Node any`), not the typed node; nil is upstream `false`. In the plain form a list is `[]any` (upstream `instanceof List`) and numbers may be `float64` or `int`. Node keys are checked in the order type, loc, structure fields, then other keys sorted (upstream: object key order). The walk follows the node and list fields of the syntax's `Structure` and skips null fields and items (upstream throws on a null required node). `lexer.structure[type].docs` is `StructureDocs(type)`. A `FieldType` with an unknown `Kind` is upstream's bad structure value ("Wrong value `123` in `Test.foo` structure definition").
- lexer: match.js's module-level `totalIterationCount` is not ported (package-level mutable state), and hitting `ITERATION_LIMIT` (15000, same counting) does not log upstream's `console.warn`; the result still carries the iteration-limit reason. `syntaxStash` (null | false | snapshot) is a pointer plus a `syntaxStashDisabled` flag. The `\0`/`\9` keyword hack uses the same regexp (`\\[09].*$`).
- lexer: generic types are `func(token *matchToken, getNextToken func(int) *matchToken, opts *definitionsyntax.Range) int` (unexported; `Config.Types` holds strings only, so a config cannot supply a function type as upstream can). `Number()` is `jsNumber`, a port of JavaScript's string-to-number conversion (whitespace trim, `""` → 0, `Infinity`, `0x`/`0o`/`0b`, NaN otherwise); `strconv.ParseFloat` alone would accept `inf`, `nan`, hex floats and underscores. A range bound with a unit (`RangeBound.Unit != ""`, upstream a string) skips the range check as upstream does. `math(...)` is `mathType` (the name clashes with package math).
- lexer: an-plus-b reads `getNextToken(offset + 1).type` after a lone `+`, which throws a TypeError upstream at the end of the input (e.g. `MatchType("an-plus-b", "+")`); the port treats a missing token as not an ident, so that input is a mismatch. search.js reads `item.data` of the node that starts a fragment, which throws when it is the walked root (no item); the port skips it.
- lexer: a definition syntax `String` of one code point becomes a `Token` (upstream: three UTF-16 code units including the quotes). `new Error('Unknown node type:', node.type)` keeps upstream's message, which drops the type.
- lexer: tests — `lexer-check-structure.js` "should ignore properties from prototype" and "node should be an object" are skipped (no prototypes; a plain node is a map). Values the fixtures give as strings use the `...String` methods. The definitionsyntax tests that read the lexer's dictionaries ("parse", "round trip / lexer dictionary") are `TestDefinitionSyntaxParseLexer`/`TestDefinitionSyntaxGenerateLexer` in the root package, with `forEachTest`, `forEachAtrulePreludeTest` and `forEachAtruleDescriptorTest` in `fixture_definition_syntax_test.go` (descriptors in name order). `exports.js` checks names by referencing the Go counterparts. `FuzzMatchProperty` and `TestLexerConcurrentMatch` are Go only.
- data: `lib/data.js` and `lib/data-patch.js` → `data.go`: `data/mdn/{at-rules,properties,syntaxes}.json` and `data/patch.json` are embedded with `go:embed` and preprocessed exactly as upstream (same regexps; `replace` without `g` replaces the first match only) on first use by `sync.OnceValues`; the result is read-only. A missing prelude match (upstream: TypeError reading `[1]` of null) is an error. `lexerConfig()` adds the definitions to the default syntax's lexer keys. An at-rule's prelude `""` and descriptors `nil` are upstream null, as in `AtruleDefinition`, which gained a `MarshalJSON` writing `{prelude, descriptors}` with nulls.
