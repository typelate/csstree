// Ported from lib/syntax/config/parser.js.

package csstree

// parserConfig returns the parser configuration of the default syntax.
func parserConfig() *Config {
	return &Config{
		Contexts: map[ParseContext]ContextParser{
			ContextDefault:    func(p *Parser, _ ParseOptions) Node { return p.StyleSheet() },
			ContextStylesheet: func(p *Parser, _ ParseOptions) Node { return p.StyleSheet() },
			ContextAtrule:     func(p *Parser, _ ParseOptions) Node { return p.Atrule(false) },
			ContextAtrulePrelude: func(p *Parser, options ParseOptions) Node {
				return p.AtrulePrelude(options.Atrule)
			},
			ContextMediaQueryList: func(p *Parser, _ ParseOptions) Node { return p.MediaQueryList() },
			ContextMediaQuery:     func(p *Parser, _ ParseOptions) Node { return p.MediaQuery() },
			ContextCondition: func(p *Parser, options ParseOptions) Node {
				return p.Condition(options.Kind)
			},
			ContextRule:         func(p *Parser, _ ParseOptions) Node { return p.Rule() },
			ContextSelectorList: func(p *Parser, _ ParseOptions) Node { return p.SelectorList() },
			ContextSelector:     func(p *Parser, _ ParseOptions) Node { return p.Selector() },
			ContextBlock: func(p *Parser, _ ParseOptions) Node {
				return p.Block(true)
			},
			ContextDeclarationList: func(p *Parser, _ ParseOptions) Node { return p.DeclarationList() },
			ContextDeclaration:     func(p *Parser, _ ParseOptions) Node { return p.Declaration() },
			ContextValue:           func(p *Parser, _ ParseOptions) Node { return p.Value() },
		},
		Features: map[string]map[string]FeatureParser{
			"supports": {
				"selector": func(p *Parser) Node {
					return p.Selector()
				},
			},
			"container": {
				"style": func(p *Parser) Node {
					return p.Declaration()
				},
			},
		},
		Scope:  scopes(),
		Atrule: atrules(),
		Pseudo: pseudos(),
		Node:   nodeConfigs(),
	}
}
