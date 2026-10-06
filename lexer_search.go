// Ported from lib/lexer/search.js.

package csstree

// Fragment is a run of sibling nodes that matched a syntax: Nodes are
// copied from Parent, the list holding them.
type Fragment struct {
	Parent *List[Node]
	Nodes  *List[Node]
}

func getFirstMatchNode(matchNode *MatchNode) Node {
	if matchNode.Match == nil {
		return matchNode.Node
	}

	return getFirstMatchNode(matchNode.Match[0])
}

func getLastMatchNode(matchNode *MatchNode) Node {
	if matchNode.Match == nil {
		return matchNode.Node
	}

	return getLastMatchNode(matchNode.Match[len(matchNode.Match)-1])
}

func matchFragments(lexer *Lexer, ast Node, match *MatchResult, typ, name string) ([]Fragment, error) {
	var fragments []Fragment
	var walkErr error

	var findFragments func(matchNode *MatchNode)
	findFragments = func(matchNode *MatchNode) {
		if matchNode.Syntax != nil &&
			matchNode.Syntax.Type() == typ &&
			syntaxName(matchNode.Syntax) == name {
			start := getFirstMatchNode(matchNode)
			end := getLastMatchNode(matchNode)

			err := lexer.syntax.Walk(ast, WalkOptions{
				Enter: func(_ *WalkContext, node Node, item *ListItem[Node], list *List[Node]) WalkAction {
					// upstream reads item.data and throws when start is the
					// root, which has no item
					if node == start && item != nil {
						nodes := new(List[Node])

						for {
							nodes.AppendData(item.Data)

							if item.Data == end {
								break
							}

							item = item.Next
							if item == nil {
								break
							}
						}

						fragments = append(fragments, Fragment{
							Parent: list,
							Nodes:  nodes,
						})
					}
					return WalkContinue
				},
			})
			if err != nil && walkErr == nil {
				walkErr = err
			}
		}

		if matchNode.Match != nil {
			for _, child := range matchNode.Match {
				findFragments(child)
			}
		}
	}

	if match.Matched != nil {
		findFragments(match.Matched)
	}

	return fragments, walkErr
}
