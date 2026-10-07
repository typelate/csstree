// Ported from lib/lexer/match.js.

package csstree

import (
	"regexp"
	"strings"

	"github.com/typelate/csstree/definitionsyntax"
	"github.com/typelate/csstree/tokenizer"
)

// stackItemKind is the kind of a match stack item.
type stackItemKind uint8

const (
	stubItem stackItemKind = iota
	tokenItem
	openSyntaxItem
	closeSyntaxItem
)

// exitReason is why the matcher stopped; the zero value is that it has
// not.
type exitReason uint8

const (
	exitMatch exitReason = iota + 1
	exitMismatch
	exitIterationLimit
)

// iterationLimitMessage is upstream's reason for exitIterationLimit.
const iterationLimitMessage = "Maximum iteration number exceeded (please fill an issue on https://github.com/csstree/csstree/issues)"

// message returns upstream's text of the reason (upstream the reason is
// that text).
func (r exitReason) message() string {
	switch r {
	case exitMatch:
		return "Match"
	case exitMismatch:
		return "Mismatch"
	case exitIterationLimit:
		return iterationLimitMessage
	}

	return ""
}

const iterationLimit = 15000

// keywordHack matches the \0 and \9 hacks at the end of a keyword: upstream
// /\\[09].*$/, where "." matches no line terminator and "$" only matches at
// the end of the value.
var keywordHack = regexp.MustCompile(`\\[09][^\n\r\x{2028}\x{2029}]*$`)

// matchSyntaxes resolves the types and properties a match graph refers to
// (upstream the syntaxes argument: an object with types and properties
// dictionaries, usually the lexer).
type matchSyntaxes interface {
	// lookupMatch returns the match graph of the type (isType) or property
	// named name (upstream dictSyntax.matchRef || dictSyntax.match), nil
	// when there is none.
	lookupMatch(isType bool, name string) (*matchGraphNode, error)
}

// matchStackItem is an entry of the match stack: a matched token or the
// opening or closing of a syntax.
type matchStackItem struct {
	kind   stackItemKind
	syntax definitionsyntax.Node
	token  *matchToken
	prev   *matchStackItem
}

type syntaxStackItem struct {
	syntax         definitionsyntax.Node
	bounds         *definitionsyntax.Range
	percentSibling bool
	prev           *syntaxStackItem
}

type thenStackItem struct {
	nextState   *matchGraphNode
	matchStack  *matchStackItem
	syntaxStack *syntaxStackItem
	prev        *thenStackItem
}

type elseStackItem struct {
	nextState   *matchGraphNode
	matchStack  *matchStackItem
	syntaxStack *syntaxStackItem
	thenStack   *thenStackItem
	tokenIndex  int
	prev        *elseStackItem
}

// matchResult is the result of internalMatch, matchAsList and matchAsTree.
// match is set by internalMatch (nil unless the value matched), list by
// matchAsList and tree by matchAsTree.
type matchResult struct {
	tokens       []matchToken
	reason       exitReason
	iterations   int
	match        *matchStackItem
	list         []matchListItem
	tree         *MatchNode
	longestMatch int
}

// matchListItem is an entry of matchAsList's result: the opening or
// closing of a syntax, or a token.
type matchListItem struct {
	kind    stackItemKind
	syntax  definitionsyntax.Node
	isToken bool
	token   string
	node    Node
}

func reverseList(list *matchStackItem) *matchStackItem {
	var prev *matchStackItem
	var next *matchStackItem
	item := list

	for item != nil {
		next = item.prev
		item.prev = prev
		prev = item
		item = next
	}

	return prev
}

func isContextEdgeDelim(token *matchToken) bool {
	if token.typ != tokenizer.Delim {
		return false
	}

	// Fix matching for unicode-range: U+30??, U+FF00-FF9F
	// Probably we need to check out previous match instead
	return token.value != "?"
}

func isCommaContextStart(token *matchToken) bool {
	if token == nil {
		return true
	}

	return token.typ == tokenizer.Comma ||
		token.typ == tokenizer.Function ||
		token.typ == tokenizer.LeftParenthesis ||
		token.typ == tokenizer.LeftSquareBracket ||
		token.typ == tokenizer.LeftCurlyBracket ||
		isContextEdgeDelim(token)
}

func isCommaContextEnd(token *matchToken) bool {
	if token == nil {
		return true
	}

	return token.typ == tokenizer.RightParenthesis ||
		token.typ == tokenizer.RightSquareBracket ||
		token.typ == tokenizer.RightCurlyBracket ||
		(token.typ == tokenizer.Delim && token.value == "/")
}

// typeOpts returns the opts of a Type syntax node (nil for other nodes,
// whose opts upstream are undefined).
func typeOpts(syntax definitionsyntax.Node) *definitionsyntax.Range {
	if node, ok := syntax.(*definitionsyntax.Type); ok {
		return node.Opts
	}

	return nil
}

// internalMatch runs the matcher. It fails where upstream throws: on a
// reference to an unknown type or property and when building a referenced
// match graph fails.
func internalMatch(tokens []matchToken, state *matchGraphNode, syntaxes matchSyntaxes) (*matchResult, error) {
	var syntaxStack *syntaxStackItem
	var thenStack *thenStackItem
	var elseStack *elseStackItem

	// nil, !syntaxStashDisabled – stashing allowed, nothing stashed
	// nil, syntaxStashDisabled – stashing disabled, nothing stashed
	// anything else – fail stashable syntaxes, some syntax stashed
	// (upstream: null, false, a state snapshot)
	var syntaxStash *elseStackItem
	syntaxStashDisabled := false

	iterationCount := 0 // count iterations and prevent infinite loop
	var reason exitReason

	var token *matchToken
	tokenIndex := -1
	longestMatch := 0
	matchStack := &matchStackItem{
		kind:   stubItem,
		syntax: nil,
		token:  nil,
		prev:   nil,
	}

	moveToNextToken := func() {
		for {
			tokenIndex++
			token = nil
			if tokenIndex < len(tokens) {
				token = &tokens[tokenIndex]
			}
			if token == nil || (token.typ != tokenizer.WhiteSpace && token.typ != tokenizer.Comment) {
				break
			}
		}
	}

	stateSnapshotFromSyntax := func(nextState *matchGraphNode, prev *elseStackItem) *elseStackItem {
		return &elseStackItem{
			nextState:   nextState,
			matchStack:  matchStack,
			syntaxStack: syntaxStack,
			thenStack:   thenStack,
			tokenIndex:  tokenIndex,
			prev:        prev,
		}
	}

	pushThenStack := func(nextState *matchGraphNode) {
		thenStack = &thenStackItem{
			nextState:   nextState,
			matchStack:  matchStack,
			syntaxStack: syntaxStack,
			prev:        thenStack,
		}
	}

	pushElseStack := func(nextState *matchGraphNode) {
		elseStack = stateSnapshotFromSyntax(nextState, elseStack)
	}

	addTokenToMatch := func() {
		matchStack = &matchStackItem{
			kind:   tokenItem,
			syntax: state.syntax,
			token:  token,
			prev:   matchStack,
		}

		moveToNextToken()
		syntaxStash = nil
		syntaxStashDisabled = false

		if tokenIndex > longestMatch {
			longestMatch = tokenIndex
		}
	}

	openSyntax := func() {
		bounds := typeOpts(state.syntax)
		if bounds == nil && syntaxStack != nil {
			bounds = syntaxStack.bounds
		}
		syntaxStack = &syntaxStackItem{
			syntax:         state.syntax,
			bounds:         bounds,
			percentSibling: state.percentSibling,
			prev:           syntaxStack,
		}

		matchStack = &matchStackItem{
			kind:   openSyntaxItem,
			syntax: state.syntax,
			token:  matchStack.token,
			prev:   matchStack,
		}
	}

	closeSyntax := func() {
		if matchStack.kind == openSyntaxItem {
			matchStack = matchStack.prev
		} else {
			matchStack = &matchStackItem{
				kind:   closeSyntaxItem,
				syntax: syntaxStack.syntax,
				token:  matchStack.token,
				prev:   matchStack,
			}
		}

		syntaxStack = syntaxStack.prev
	}

	moveToNextToken()

	for reason == 0 {
		iterationCount++
		if iterationCount >= iterationLimit {
			break
		}

		switch state.kind {
		case graphMatch:
			if thenStack == nil {
				// turn to MISMATCH when some tokens left unmatched
				if token != nil {
					// doesn't mismatch if just one token left and it's an IE hack
					if tokenIndex != len(tokens)-1 || (token.value != `\0` && token.value != `\9`) {
						state = mismatchState
						break
					}
				}

				// break the main loop, return a result - MATCH
				reason = exitMatch
				break
			}

			// go to next syntax (`then` branch)
			state = thenStack.nextState

			// check match is not empty
			if state == disallowEmptyState {
				if thenStack.matchStack == matchStack {
					state = mismatchState
					break
				} else {
					state = matchState
				}
			}

			// close syntax if needed
			for thenStack.syntaxStack != syntaxStack {
				closeSyntax()
			}

			// pop stack
			thenStack = thenStack.prev

		case graphMismatch:
			// when some syntax is stashed
			if syntaxStash != nil {
				// there is no else branches or a branch reduce match stack
				if elseStack == nil || tokenIndex > elseStack.tokenIndex {
					// restore state from the stash
					elseStack = syntaxStash
					syntaxStash = nil
					syntaxStashDisabled = true // disable stashing
				}
			} else if elseStack == nil {
				// no else branches -> break the main loop
				// return a result - MISMATCH
				reason = exitMismatch
				break
			}

			// go to next syntax (`else` branch)
			state = elseStack.nextState

			// restore all the rest stack states
			thenStack = elseStack.thenStack
			syntaxStack = elseStack.syntaxStack
			matchStack = elseStack.matchStack
			tokenIndex = elseStack.tokenIndex
			token = nil
			if tokenIndex < len(tokens) {
				token = &tokens[tokenIndex]
			}

			// pop stack
			elseStack = elseStack.prev

		case graphMatchGraph:
			state = state.match

		case graphIf:
			// IMPORTANT: else stack push must go first,
			// since it stores the state of thenStack before changes
			if state.els != mismatchState {
				pushElseStack(state.els)
			}

			if state.then != matchState {
				pushThenStack(state.then)
			}

			state = state.match

		case graphMatchOnce:
			state = &matchGraphNode{
				kind:  graphMatchOnceBuffer,
				once:  state,
				index: 0,
				mask:  0,
			}

		case graphMatchOnceBuffer:
			terms := state.once.terms

			if state.index == len(terms) {
				// no matches at all or it's required all terms to be matched
				if state.mask == 0 || state.once.all {
					state = mismatchState
					break
				}

				// a partial match is ok
				state = matchState
				break
			}

			// all terms are matched
			if state.mask == (1<<len(terms))-1 {
				state = matchState
				break
			}

			for buffer := state; buffer.index < len(terms); buffer.index++ {
				matchFlag := 1 << buffer.index

				if buffer.mask&matchFlag == 0 {
					// IMPORTANT: else stack push must go first,
					// since it stores the state of thenStack before changes
					pushElseStack(buffer)
					pushThenStack(&matchGraphNode{
						kind: graphAddMatchOnce,
						once: buffer.once,
						mask: buffer.mask | matchFlag,
					})

					// match
					state = terms[buffer.index]
					buffer.index++
					break
				}
			}

		case graphAddMatchOnce:
			state = &matchGraphNode{
				kind:  graphMatchOnceBuffer,
				once:  state.once,
				index: 0,
				mask:  state.mask,
			}

		case graphEnum:
			if token != nil {
				name := toLowerCase(token.value)

				// drop \0 and \9 hack from keyword name
				if strings.Contains(name, `\`) {
					name = keywordHack.ReplaceAllString(name, "")
				}

				if next, ok := state.enumMap[name]; ok {
					state = next
					break
				}

				// the value with escapes decoded, see tokenNameIs
				if next, ok := enumLookup(state.enumMap, decodedNames(token)); ok {
					state = next
					break
				}
			}

			state = mismatchState

		case graphGeneric:
			var bounds *definitionsyntax.Range
			if syntaxStack != nil {
				bounds = syntaxStack.bounds
			}
			// math functions are type checked (Values 4 §10.9)
			percentSibling := syntaxStack != nil && syntaxStack.percentSibling
			rest := tokens[tokenIndex:]
			consumed, handled := matchMathFunction(state.generic.calc, percentSibling, rest)
			if !handled {
				consumed = state.generic.fn(rest, bounds)
			}
			lastTokenIndex := tokenIndex + consumed

			if lastTokenIndex > tokenIndex {
				for tokenIndex < lastTokenIndex {
					addTokenToMatch()
				}

				state = matchState
			} else {
				state = mismatchState
			}

		case graphType, graphProperty:
			isType := state.kind == graphType
			var dictSyntax *matchGraphNode
			if syntaxes != nil {
				var err error
				dictSyntax, err = syntaxes.lookupMatch(isType, state.name)
				if err != nil {
					return nil, err
				}
			}

			if dictSyntax == nil {
				kind := KindProperty
				if isType {
					kind = KindType
				}
				return nil, newBadSyntaxReferenceError(kind, state.name)
			}

			// stash a syntax for types with low priority
			if !syntaxStashDisabled && token != nil && isType {
				// https://drafts.csswg.org/css-values-4/#custom-idents
				// When parsing positionally-ambiguous keywords in a property value, a <custom-ident> production
				// can only claim the keyword if no other unfulfilled production can claim it.
				lowPriorityCustomIdent := state.name == "custom-ident" && token.typ == tokenizer.Ident

				// https://drafts.csswg.org/css-values-4/#lengths
				// ... if a `0` could be parsed as either a <number> or a <length> in a property (such as line-height),
				// it must parse as a <number>
				lowPriorityLength := state.name == "length" && token.value == "0"

				lowPriorityMatching := lowPriorityCustomIdent || lowPriorityLength

				if lowPriorityMatching {
					if syntaxStash == nil {
						syntaxStash = stateSnapshotFromSyntax(state, elseStack)
					}

					state = mismatchState
					break
				}
			}

			openSyntax()
			state = dictSyntax

		case graphKeyword:
			name := state.name

			if token != nil {
				keywordName := token.value

				// drop \0 and \9 hack from keyword name
				if strings.Contains(keywordName, `\`) {
					keywordName = keywordHack.ReplaceAllString(keywordName, "")
				}

				if asciiEqualFold(keywordName, name) || tokenNameIs(token, name) {
					addTokenToMatch()
					state = matchState
					break
				}
			}

			state = mismatchState

		case graphAtKeyword, graphFunction:
			if token != nil && (asciiEqualFold(token.value, state.name) || tokenNameIs(token, state.name)) {
				addTokenToMatch()
				state = matchState
				break
			}

			state = mismatchState

		case graphToken:
			if token != nil && token.value == state.value {
				addTokenToMatch()
				state = matchState
				break
			}

			state = mismatchState

		case graphComma:
			if token != nil && token.typ == tokenizer.Comma {
				if isCommaContextStart(matchStack.token) {
					state = mismatchState
				} else {
					addTokenToMatch()
					if isCommaContextEnd(token) {
						state = mismatchState
					} else {
						state = matchState
					}
				}
			} else {
				if isCommaContextStart(matchStack.token) || isCommaContextEnd(token) {
					state = matchState
				} else {
					state = mismatchState
				}
			}

		case graphString:
			var str strings.Builder
			lastTokenIndex := tokenIndex

			for ; lastTokenIndex < len(tokens) && str.Len() < len(state.value); lastTokenIndex++ {
				str.WriteString(tokens[lastTokenIndex].value)
			}

			if asciiEqualFold(str.String(), state.value) {
				for tokenIndex < lastTokenIndex {
					addTokenToMatch()
				}

				state = matchState
			} else {
				state = mismatchState
			}

		default:
			return nil, &UnknownNodeTypeError{Type: state.kind.String()}
		}
	}

	switch reason {
	case 0:
		// upstream also logs "[csstree-match] BREAK after 15000 iterations"
		// with console.warn
		reason = exitIterationLimit
		matchStack = nil

	case exitMatch:
		for syntaxStack != nil {
			closeSyntax()
		}

	default:
		matchStack = nil
	}

	return &matchResult{
		tokens:       tokens,
		reason:       reason,
		iterations:   iterationCount,
		match:        matchStack,
		longestMatch: longestMatch,
	}, nil
}

func matchAsList(tokens []matchToken, matchGraph *matchGraphNode, syntaxes matchSyntaxes) (*matchResult, error) {
	matchResult, err := internalMatch(tokens, matchGraph, syntaxes)
	if err != nil {
		return nil, err
	}

	if matchResult.match != nil {
		item := reverseList(matchResult.match).prev

		matchResult.list = []matchListItem{}

		for item != nil {
			switch item.kind {
			case openSyntaxItem, closeSyntaxItem:
				matchResult.list = append(matchResult.list, matchListItem{
					kind:   item.kind,
					syntax: item.syntax,
				})

			default:
				matchResult.list = append(matchResult.list, matchListItem{
					isToken: true,
					token:   item.token.value,
					node:    item.token.node,
				})
			}

			item = item.prev
		}
	}

	return matchResult, nil
}

func matchAsTree(tokens []matchToken, matchGraph *matchGraphNode, syntaxes matchSyntaxes) (*matchResult, error) {
	matchResult, err := internalMatch(tokens, matchGraph, syntaxes)
	if err != nil {
		return nil, err
	}

	if matchResult.match == nil {
		return matchResult, nil
	}

	item := matchResult.match
	host := &MatchNode{
		Match: []*MatchNode{},
	}
	if matchGraph.ref != nil {
		host.Syntax = matchGraph.ref
	}
	matchResult.tree = host
	hostStack := []*MatchNode{host}

	// revert a list and start with 2nd item since 1st is a stub item
	item = reverseList(item).prev

	// build a tree
	for item != nil {
		switch item.kind {
		case openSyntaxItem:
			host.Match = append(host.Match, &MatchNode{
				Syntax: item.syntax,
				Match:  []*MatchNode{},
			})
			host = host.Match[len(host.Match)-1]
			hostStack = append(hostStack, host)

		case closeSyntaxItem:
			hostStack = hostStack[:len(hostStack)-1]
			host = hostStack[len(hostStack)-1]

		default:
			leaf := &MatchNode{
				Token: item.token.value,
				Node:  item.token.node,
			}
			if item.syntax != nil {
				leaf.Syntax = item.syntax
			}
			host.Match = append(host.Match, leaf)
		}

		item = item.prev
	}

	return matchResult, nil
}
