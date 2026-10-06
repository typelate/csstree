// Ported from lib/syntax/config/generator.js.

package csstree

// generatorConfig returns the generator configuration of the default
// syntax: every node type, of which the generator uses Generate.
func generatorConfig() *Config {
	return &Config{
		Node: nodeConfigs(),
	}
}
