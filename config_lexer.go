// Ported from lib/syntax/config/lexer.js.

package csstree

// lexerConfig returns the lexer configuration of the default syntax.
//
// TODO: add upstream's `...definitions` (types, properties, atrules and
// units from lib/data.js) once data.go is ported.
//
//lint:ignore U1000 used by the lexer and walker, which are not ported yet
func lexerConfig() *Config {
	return &Config{
		Generic:         true,
		CSSWideKeywords: cssWideKeywords(),
		Node:            nodeConfigs(),
	}
}
