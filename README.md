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
- Forks can extend parse contexts, scopes, at-rules, pseudo-classes, features
  and node generators, but not replace a built-in node's parse function.
- Upstream bugs that throw or overflow in JavaScript are fixed: sources over
  16 MiB, a bad `url(` ending in an escape, `an+b` matching a lone `+`.
- A walk with `Visit` set honours `WalkBreak` and `WalkSkip` from its
  handlers, as upstream's documentation says; upstream ignores `walk.break`
  and `walk.skip` when `visit` is set.

## License

MIT, like upstream; see [LICENSE](LICENSE). The port is copyright Christopher
Hunter; the csstree code it is ported from is copyright Roman Dvornov, whose
notice the MIT license requires keeping. The embedded data in `data/mdn` is
from mdn-data (CC0, see `data/mdn/LICENSE`).
