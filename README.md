# csstree

A Go port of [csstree](https://github.com/csstree/csstree) v3.2.1: a CSS
tokenizer, parser (CSS → AST), walker, generator (AST → CSS, with source maps)
and lexer (validating values against the CSS grammar from
[mdn-data](https://github.com/mdn/data)).

The port follows upstream file by file (each Go file names the upstream file
it came from), and runs upstream's test suite against upstream's fixtures.

```go
ast, err := csstree.Parse(`.a { color: red }`, csstree.ParseOptions{})
if err != nil {
	return err
}

css, err := csstree.Generate(ast, csstree.GenerateOptions{})

result := csstree.DefaultLexer().MatchPropertyString("color", "red")
if result.Error != nil {
	// the value does not match the property's syntax
}
```

See `Example` in [example_test.go](example_test.go) for walking an AST.

## Packages

- `github.com/typelate/csstree`: AST, parser, generator, walker, lexer, and fork
- `github.com/typelate/csstree/tokenizer`: the CSS tokenizer and token stream
- `github.com/typelate/csstree/definitionsyntax`: parser and generator for
  CSS value definition syntax (`<length> | auto`)

## Differences from csstree

- Offsets and columns in locations, errors and source maps count bytes of
  UTF-8, not UTF-16 code units.
- The AST is typed: each node type is a Go struct implementing `Node`, children
  are `*List[Node]`, and JSON encoding matches upstream's. There is no
  `list: false` (array children) mode.
- Functions that throw upstream return an `error`. Lexer matches report
  failures in `MatchResult.Error`.
- Only syntax errors fall back to a `Raw` node. Any other panic in a parse
  or generate function, an extension or a callback is a bug: `Parse` and
  `Generate` return it as an `*InternalError` holding the panic value and
  stack, where upstream's `parseWithFallback` catches every exception.
- Forks can extend parse contexts, scopes, at-rules, pseudo-classes, features
  and node generators, but not replace a built-in node's parse function.
- Upstream bugs that throw or overflow in JavaScript are fixed: sources over
  16 MiB, a bad `url(` ending in an escape, `an+b` matching a lone `+`.
- A walk with `Visit` set honours `WalkBreak` and `WalkSkip` from its
  handlers, as upstream's documentation says; upstream ignores `walk.break`
  and `walk.skip` when `visit` is set.

### Where the port follows the specs instead of upstream

Tokenizer (CSS Syntax 3):

- U+0000 and invalid UTF-8 read as U+FFFD; token offsets still index the
  original bytes, and the `Decode*` functions return U+FFFD.
- A bad-string ends before the newline that ends it, and bad-url remnants
  resume right after an escape.
- Only U+FEFF is skipped as a byte order mark.

Parser and generator:

- Nested style rules don't need `&`: `.a{.b{}}`, `.a{> .b{}}` and
  `.a{color:red; .b{} color:blue}` give nested rules.
- Media and container queries accept `=` ranges (`(width = 600px)`); mixed
  direction ranges and conditions mixing `and`/`or` at one level are errors.
- The column combinator `||` parses, and attribute modifiers must be `i` or
  `s`.
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

Definition syntax:

- `{0}` means exactly zero (`definitionsyntax.Unbounded` marks no upper
  bound), `{B,A}` with B < A is an error, and range bounds accept U+2212 `−`.

## License

MIT, like upstream; see [LICENSE](LICENSE). The port is copyright Christopher
Hunter; the csstree code it is ported from is copyright Roman Dvornov, whose
notice the MIT license requires keeping. The embedded data in `data/mdn` is
from mdn-data (CC0, see `data/mdn/LICENSE`).
