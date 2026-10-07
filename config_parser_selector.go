// Ported from lib/syntax/config/parser-selector.js.

package csstree

// parserSelectorConfig returns the configuration of the selector-only
// parser (ParseSelector).
func parserSelectorConfig() *Config {
	return &Config{
		Contexts: map[ParseContext]ContextParser{
			ContextDefault:      func(p *Parser, _ ParseOptions) Node { return p.SelectorList() },
			ContextSelectorList: func(p *Parser, _ ParseOptions) Node { return p.SelectorList() },
			ContextSelector:     func(p *Parser, _ ParseOptions) Node { return p.Selector() },
		},
		Scope:  map[string]*Recognizer{"Selector": selectorScope()},
		Atrule: map[string]AtruleParser{},
		Pseudo: pseudos(),
		Node:   nodeConfigs(),
	}
}
