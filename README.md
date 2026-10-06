# csstree

A Go port of [csstree](https://github.com/csstree/csstree) v3.2.1: a CSS
tokenizer, parser (CSS → AST), walker, generator (AST → CSS, with source maps)
and lexer (validating values against the CSS grammar from
[mdn-data](https://github.com/mdn/data)).

The port follows upstream file by file, and runs upstream's test suite against
upstream's fixtures. Differences from the JavaScript library are listed in
[PORTING.md](PORTING.md).

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

## License

MIT, like upstream; see [LICENSE](LICENSE). The embedded data in `data/mdn` is
from mdn-data (CC0, see `data/mdn/LICENSE`).
