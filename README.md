# csstree

A Go port of [csstree](https://github.com/csstree/csstree) v3.2.1: a CSS
tokenizer, parser (CSS → AST), walker, generator (AST → CSS, with source maps)
and lexer (validating values against the CSS grammar from
[mdn-data](https://github.com/mdn/data)).

The port follows upstream file by file (each Go file names the upstream file
it came from, or says it is Go only), and runs upstream's test suite against
upstream's fixtures.

```go
ast, err := csstree.Parse(`.a { color: red }`, csstree.ParseOptions{})
if err != nil {
	return err
}

err = csstree.Walk(ast, csstree.WalkOptions{
	Visit: "Declaration",
	Enter: func(_ *csstree.WalkContext, node csstree.Node, _ *csstree.ListItem[csstree.Node], _ *csstree.List[csstree.Node]) csstree.WalkAction {
		fmt.Println(node.(*csstree.Declaration).Property)
		return csstree.WalkContinue
	},
})
if err != nil {
	return err
}

css, err := csstree.Generate(ast, csstree.GenerateOptions{})
if err != nil {
	return err
}

_, err = csstree.DefaultLexer().MatchPropertyString("color", "10px")
var mismatch *csstree.SyntaxMatchError
if errors.As(err, &mismatch) {
	// the value does not match the syntax of color
}
```

`ParseOptions.Context` parses other parts of a style sheet, such as a
declaration value (`csstree.ContextValue`) or a selector
(`csstree.ContextSelector`). The package documentation lists every context
and the node type it returns. The `Example` functions in the
[example_test.go](example_test.go) files show parsing, generating with a
source map, walking, finding nodes, forking the syntax, lexer matching and
the tokenizer and definition syntax packages.

## Packages

- `github.com/typelate/csstree`: the AST, parser, walker, generator (with
  source maps), lexer, and `Fork` for extending the syntax.
- `github.com/typelate/csstree/tokenizer`: the CSS tokenizer, the token
  stream the parser reads, and offset to line and column conversion.
- `github.com/typelate/csstree/definitionsyntax`: parser, generator and
  walker for CSS value definition syntax (`<length> | auto`), which the lexer
  matches values against.

Internal packages, for contributors:

- `internal/nodegen` and `internal/cmd/nodegen`: the generator of
  `nodes_gen.go` (see Development).
- `internal/jsnum`: number parsing and formatting as JavaScript does them,
  which the lexer and definition syntax need to match upstream.
- `internal/fixturetest`: loaders for the JSON fixtures that tests of more
  than one package read.

## Differences from csstree

- Offsets and columns in locations, errors and source maps count bytes of
  UTF-8, not UTF-16 code units.
- The AST is typed: each node type is a Go struct implementing `Node`, children
  are `*List[Node]`, and JSON encoding matches upstream's. There is no
  `list: false` (array children) mode.
- Functions that throw upstream return an `error`, and so do lexer matches
  where upstream reports failures in the result's `error`.
- Only syntax errors fall back to a `Raw` node. Any other panic in a parse
  or generate function, an extension or a callback is a bug: `Parse` and
  `Generate` return it as an `*InternalError` holding the panic value and
  stack, where upstream's `parseWithFallback` catches every exception.
- Forks can extend parse contexts, scopes, at-rules, pseudo-classes,
  features, node types and node generators, but not replace a built-in
  node's parse function.
- Upstream bugs that throw or overflow in JavaScript are fixed: sources over
  16 MiB, a bad `url(` ending in an escape, `an+b` matching a lone `+`.
- A walk with `Visit` set honours `WalkBreak` and `WalkSkip` from its
  handlers, as upstream's documentation says; upstream ignores `walk.break`
  and `walk.skip` when `visit` is set.

### Go API

- The AST is typed. Node types are structs, a pointer to one implements
  `Node`, and type switches select them. Optional strings are `""` where
  upstream has null.
- Errors are values. `Parse`, `Generate` and the lexer's `Match*` methods
  return an `error` next to their result, and `Walk` returns an `error`.
  The error types (`*SyntaxError`, `*SyntaxMatchError`,
  `*SyntaxReferenceError`, `*InternalError`, ...) and sentinel errors work
  with `errors.Is` and `errors.As`.
- A `Lexer` does not change once created. For other definitions, build a
  lexer with `Syntax.NewLexer` or fork the syntax with `Fork`.
- `Syntax`, `Lexer`, `DefaultSyntax()`, `DefaultLexer()` and the package
  functions are safe for concurrent use. `Generate`, `GenerateWithSourceMap`,
  `Find*`, `Clone`, `ToPlainObject` and lexer matching only read an AST, so
  they may run on one AST at the same time; `Walk` and changes to an AST need
  exclusive access to it. `List`, `Parser` and `SourceMapGenerator` values
  are not safe for concurrent use, except that `List.All` and
  `List.Backward` may read a list nobody changes.
- Parse contexts, generate modes and lexer syntax kinds are typed constants
  (`ContextValue`, `ModeSpec`, `KindProperty`, ...). Option structs have
  useful zero values, so the options that upstream turns on by default
  are `NoParse*` fields.
- `GenerateWithSourceMap` returns the CSS and a `*SourceMapGenerator`;
  upstream's `generate` returns `{ css, map }` when `sourceMap` is set.

### Where the port follows the specs instead of upstream

Tokenizer (CSS Syntax 3):

- U+0000 and invalid UTF-8 read as U+FFFD; token offsets still index the
  original bytes, and the `Decode*` functions return U+FFFD.
- A bad-string ends before the newline that ends it, and bad-url remnants
  resume right after an escape.
- Only U+FEFF is skipped as a byte order mark.
- A `\` at the end of the source is an escape, so `a\` is one ident.
- A name is `url` by its value: `u\72l(foo)` is a url token.
- `DecodeURL` keeps a whitespace that a `\` escapes at the end of a url,
  and decodes a url of only whitespace to the empty string.

Parser and generator:

- Nested style rules don't need `&`: `.a{.b{}}`, `.a{> .b{}}` and
  `.a{color:red; .b{} color:blue}` give nested rules.
- Media and container queries accept `=` ranges (`(width = 600px)`); mixed
  direction ranges and conditions mixing `and`/`or` at one level are errors.
- The column combinator `||` parses, and attribute modifiers must be `i` or
  `s`.
- A functional pseudo with only whitespace or a comment inside, such as
  `:is( )`, parses with no children.
- The end of the source closes an open attribute selector: `[a=b` parses as
  `[a=b]`.
- Only a trailing `!important` leaves a declaration value
  (`red!important!important` is important with value `red!important`), and a
  `;` inside `()`, `[]` or a function stays in the value.
- Parse → generate is a fixed point after a lone `\` or a value that ends
  inside a string.

Lexer (CSS Values 4):

- Keywords, units and function names compare by their escape-decoded value
  (`\72 ed` is `red`); `<custom-ident>` rejects escaped CSS-wide keywords.
- `<number>` takes only number tokens, not `+`, `-` or idents like `e5`.
- Ranges with units are checked (`<angle [0deg,360deg]>` rejects `400deg`),
  converting between units of one type; relative lengths are checked by sign.
- `<urange>` rejects ranges above U+10FFFF or with start after end.
- Math functions (`calc()`, `min()`, `clamp()`, `round()`, trigonometric and
  others) are type checked: `width: calc(red)`, `opacity: calc(1px)` and
  `calc(1px+2px)` no longer match.
- A value with `env()` that does not match as written fails with
  `ErrEnvNotSupported` instead of a mismatch, since what `env()` stands for
  is unknown; a value that matches with it, such as
  `calc(1px + env(safe-area-inset-top))`, still matches.
- The color functions take relative colors (CSS Color 5):
  `rgb(from red r g b / alpha)` and `oklch(from red calc(l * 0.9) c h)`
  match, with the channel keywords of each function.

Definition syntax:

- `{0}` means exactly zero (`definitionsyntax.Unbounded` marks no upper
  bound), `{B,A}` with B < A is an error, and range bounds accept U+2212 `−`.

## Development

- `go generate ./...` regenerates `nodes_gen.go` from the `css` struct tags
  of the node types in `node_*.go` (with `internal/cmd/nodegen`). A test
  fails when `nodes_gen.go` is stale.
- `go test ./...` runs upstream's test suite against upstream's fixtures,
  the Go-only tests and the examples. `go test -race ./...` also checks the
  concurrency tests.
- Fuzz targets:

  ```
  go test -run='^$' -fuzz=FuzzParse .
  go test -run='^$' -fuzz=FuzzMatchProperty .
  go test -run='^$' -fuzz=FuzzMathFunction .
  go test -run='^$' -fuzz=FuzzTokenize ./tokenizer
  ```

- Benchmarks of tokenizing, parsing, generating (with and without a source
  map) and property matching: `go test -run='^$' -bench=. .`
- The fixtures in `testdata/` come from upstream csstree, except
  `testdata/fuzz`, which holds fuzz corpus entries, and
  `testdata/ahead-of-upstream`, which holds fixtures for behavior upstream
  does not have. `data/mdn` is mdn-data and `data/patch.json` is upstream's
  patch to it (`data-patch.js`) as JSON, plus the relative color syntaxes.

## License

MIT, like upstream; see [LICENSE](LICENSE). The port is copyright Christopher
Hunter; the csstree code it is ported from is copyright Roman Dvornov, whose
notice the MIT license requires keeping. The embedded data in `data/mdn` is
from mdn-data (CC0, see `data/mdn/LICENSE`).
