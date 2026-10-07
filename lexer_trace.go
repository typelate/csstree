// Ported from lib/lexer/trace.js.

package csstree

import (
	"slices"

	"github.com/typelate/csstree/definitionsyntax"
)

// isTraceSyntax reports whether a trace keeps syntax: a Type, Property or
// Keyword node, or the type or property at the root of a match.
func isTraceSyntax(syntax MatchSyntax) bool {
	switch syntax := syntax.(type) {
	case *definitionsyntax.Type, *definitionsyntax.Property, *definitionsyntax.Keyword:
		return true
	case *SyntaxRef:
		return syntax.Kind == KindType || syntax.Kind == KindProperty
	}

	return false
}

// Trace returns the syntaxes that matched node, outermost first, keeping
// the Type, Property and Keyword ones. It returns nil when node is not in
// the match, and an empty slice when it is but no such
// syntax led to it.
func (r *MatchResult) Trace(node Node) []MatchSyntax {
	if r.Matched == nil {
		return nil
	}

	// trace collects the syntaxes innermost first
	var trace []MatchSyntax

	var hasMatch func(matchNode *MatchNode) bool
	hasMatch = func(matchNode *MatchNode) bool {
		found := false
		if matchNode.Match == nil {
			found = matchNode.Node == node
			if found {
				trace = []MatchSyntax{}
			}
		} else {
			for _, child := range matchNode.Match {
				if hasMatch(child) {
					found = true
					break
				}
			}
		}

		if found && isTraceSyntax(matchNode.Syntax) {
			trace = append(trace, matchNode.Syntax)
		}

		return found
	}

	if !hasMatch(r.Matched) {
		return nil
	}
	slices.Reverse(trace)

	return trace
}

// IsType reports whether a syntax of type typ (a Type reference named
// typ) led to node in the match.
func (r *MatchResult) IsType(node Node, typ string) bool {
	return testNode(r, node, func(syntax MatchSyntax) bool {
		switch syntax := syntax.(type) {
		case *definitionsyntax.Type:
			return syntax.Name == typ
		case *SyntaxRef:
			return syntax.Kind == KindType && syntax.Name == typ
		}

		return false
	})
}

// IsProperty reports whether the syntax of property led to node in the
// match.
func (r *MatchResult) IsProperty(node Node, property string) bool {
	return testNode(r, node, func(syntax MatchSyntax) bool {
		switch syntax := syntax.(type) {
		case *definitionsyntax.Property:
			return syntax.Name == property
		case *SyntaxRef:
			return syntax.Kind == KindProperty && syntax.Name == property
		}

		return false
	})
}

// IsKeyword reports whether node matched a keyword.
func (r *MatchResult) IsKeyword(node Node) bool {
	return testNode(r, node, func(syntax MatchSyntax) bool {
		_, ok := syntax.(*definitionsyntax.Keyword)
		return ok
	})
}

func testNode(match *MatchResult, node Node, fn func(MatchSyntax) bool) bool {
	return slices.ContainsFunc(match.Trace(node), fn)
}
