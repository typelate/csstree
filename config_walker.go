// Ported from lib/syntax/config/walker.js.

package csstree

// walkerConfig returns the walker configuration of the default syntax:
// every node type, of which the walker uses Structure and WalkContext.
//
//lint:ignore U1000 used by the lexer and walker, which are not ported yet
func walkerConfig() *Config {
	return &Config{
		Node: nodeConfigs(),
	}
}
