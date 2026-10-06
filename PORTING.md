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
