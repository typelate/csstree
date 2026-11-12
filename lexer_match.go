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

// matchResult is the result of internalMatch and matchAsTree. match is
// set by internalMatch (nil unless the value matched) and tree by
// matchAsTree.
type matchResult struct {
	tokens       []matchToken
	reason       exitReason
	iterations   int
	match        *matchStackItem
	tree         *MatchNode
	longestMatch int
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

// matcher is the state of one run of the matcher (upstream the local
// variables of internalMatch).
type matcher struct {
	tokens   []matchToken
	syntaxes matchSyntaxes

	state *matchGraphNode

	token        *matchToken
	tokenIndex   int
	longestMatch int

	// colors tells which math functions of tokens are in a relative color
	colors relativeColors

	matchStack  *matchStackItem
	syntaxStack *syntaxStackItem
	thenStack   *thenStackItem
	elseStack   *elseStackItem

	// nil, !syntaxStashDisabled – stashing allowed, nothing stashed
	// nil, syntaxStashDisabled – stashing disabled, nothing stashed
	// anything else – fail stashable syntaxes, some syntax stashed
	// (upstream: null, false, a state snapshot)
	syntaxStash         *elseStackItem
	syntaxStashDisabled bool

	// the stack items, allocated a chunk at a time
	matchItems  slab[matchStackItem]
	syntaxItems slab[syntaxStackItem]
	thenItems   slab[thenStackItem]
	elseItems   slab[elseStackItem]
}

// slab hands out pointers to items it allocates a chunk at a time, each
// chunk twice the size of the one before up to maxSlabChunk items. The
// matcher allocates its stack items from slabs since it pushes many and
// drops them all when it is done.
type slab[T any] struct {
	chunk []T
}

const (
	minSlabChunk = 4
	maxSlabChunk = 128
)

// alloc returns a pointer to a new zero item.
func (s *slab[T]) alloc() *T {
	if len(s.chunk) == cap(s.chunk) {
		s.chunk = make([]T, 0, min(max(2*cap(s.chunk), minSlabChunk), maxSlabChunk))
	}
	s.chunk = s.chunk[:len(s.chunk)+1]

	return &s.chunk[len(s.chunk)-1]
}

// pushMatch pushes an item onto the match stack.
func (m *matcher) pushMatch(kind stackItemKind, syntax definitionsyntax.Node, token *matchToken) {
	item := m.matchItems.alloc()
	*item = matchStackItem{
		kind:   kind,
		syntax: syntax,
		token:  token,
		prev:   m.matchStack,
	}
	m.matchStack = item
}

func (m *matcher) moveToNextToken() {
	for {
		m.tokenIndex++
		m.token = nil
		if m.tokenIndex < len(m.tokens) {
			m.token = &m.tokens[m.tokenIndex]
		}
		if m.token == nil || (m.token.typ != tokenizer.WhiteSpace && m.token.typ != tokenizer.Comment) {
			break
		}
	}
}

func (m *matcher) stateSnapshotFromSyntax(nextState *matchGraphNode, prev *elseStackItem) *elseStackItem {
	item := m.elseItems.alloc()
	*item = elseStackItem{
		nextState:   nextState,
		matchStack:  m.matchStack,
		syntaxStack: m.syntaxStack,
		thenStack:   m.thenStack,
		tokenIndex:  m.tokenIndex,
		prev:        prev,
	}

	return item
}

func (m *matcher) pushThenStack(nextState *matchGraphNode) {
	item := m.thenItems.alloc()
	*item = thenStackItem{
		nextState:   nextState,
		matchStack:  m.matchStack,
		syntaxStack: m.syntaxStack,
		prev:        m.thenStack,
	}
	m.thenStack = item
}

func (m *matcher) pushElseStack(nextState *matchGraphNode) {
	m.elseStack = m.stateSnapshotFromSyntax(nextState, m.elseStack)
}

func (m *matcher) addTokenToMatch() {
	m.pushMatch(tokenItem, m.state.syntax, m.token)

	m.moveToNextToken()
	m.syntaxStash = nil
	m.syntaxStashDisabled = false

	if m.tokenIndex > m.longestMatch {
		m.longestMatch = m.tokenIndex
	}
}

func (m *matcher) openSyntax() {
	bounds := typeOpts(m.state.syntax)
	if bounds == nil && m.syntaxStack != nil {
		bounds = m.syntaxStack.bounds
	}
	item := m.syntaxItems.alloc()
	*item = syntaxStackItem{
		syntax:         m.state.syntax,
		bounds:         bounds,
		percentSibling: m.state.percentSibling,
		prev:           m.syntaxStack,
	}
	m.syntaxStack = item

	m.pushMatch(openSyntaxItem, m.state.syntax, m.matchStack.token)
}

func (m *matcher) closeSyntax() {
	if m.matchStack.kind == openSyntaxItem {
		m.matchStack = m.matchStack.prev
	} else {
		m.pushMatch(closeSyntaxItem, m.syntaxStack.syntax, m.matchStack.token)
	}

	m.syntaxStack = m.syntaxStack.prev
}

// internalMatch runs the matcher. It fails where upstream throws: on a
// reference to an unknown type or property and when building a referenced
// match graph fails.
func internalMatch(tokens []matchToken, state *matchGraphNode, syntaxes matchSyntaxes) (*matchResult, error) {
	m := &matcher{
		tokens:     tokens,
		syntaxes:   syntaxes,
		state:      state,
		tokenIndex: -1,
		colors:     relativeColors{tokens: tokens},
	}
	m.pushMatch(stubItem, nil, nil)

	return m.run()
}

// run matches the tokens from the state the matcher starts in.
func (m *matcher) run() (*matchResult, error) {
	iterationCount := 0 // count iterations and prevent infinite loop
	var reason exitReason

	m.moveToNextToken()

	for reason == 0 {
		iterationCount++
		if iterationCount >= iterationLimit {
			break
		}

		switch m.state.kind {
		case graphMatch:
			if m.thenStack == nil {
				// turn to MISMATCH when some tokens left unmatched
				if m.token != nil {
					// doesn't mismatch if just one token left and it's an IE hack
					if m.tokenIndex != len(m.tokens)-1 || (m.token.value != `\0` && m.token.value != `\9`) {
						m.state = mismatchState
						break
					}
				}

				// break the main loop, return a result - MATCH
				reason = exitMatch
				break
			}

			// go to next syntax (`then` branch)
			m.state = m.thenStack.nextState

			// check match is not empty
			if m.state == disallowEmptyState {
				if m.thenStack.matchStack == m.matchStack {
					m.state = mismatchState
					break
				} else {
					m.state = matchState
				}
			}

			// close syntax if needed
			for m.thenStack.syntaxStack != m.syntaxStack {
				m.closeSyntax()
			}

			// pop stack
			m.thenStack = m.thenStack.prev

		case graphMismatch:
			// when some syntax is stashed
			if m.syntaxStash != nil {
				// there is no else branches or a branch reduce match stack
				if m.elseStack == nil || m.tokenIndex > m.elseStack.tokenIndex {
					// restore state from the stash
					m.elseStack = m.syntaxStash
					m.syntaxStash = nil
					m.syntaxStashDisabled = true // disable stashing
				}
			} else if m.elseStack == nil {
				// no else branches -> break the main loop
				// return a result - MISMATCH
				reason = exitMismatch
				break
			}

			// go to next syntax (`else` branch)
			m.state = m.elseStack.nextState

			// restore all the rest stack states
			m.thenStack = m.elseStack.thenStack
			m.syntaxStack = m.elseStack.syntaxStack
			m.matchStack = m.elseStack.matchStack
			m.tokenIndex = m.elseStack.tokenIndex
			m.token = nil
			if m.tokenIndex < len(m.tokens) {
				m.token = &m.tokens[m.tokenIndex]
			}

			// pop stack
			m.elseStack = m.elseStack.prev

		case graphMatchGraph:
			m.state = m.state.match

		case graphIf:
			// IMPORTANT: else stack push must go first,
			// since it stores the state of thenStack before changes
			if m.state.els != mismatchState {
				m.pushElseStack(m.state.els)
			}

			if m.state.then != matchState {
				m.pushThenStack(m.state.then)
			}

			m.state = m.state.match

		case graphMatchOnce:
			m.state = &matchGraphNode{
				kind:  graphMatchOnceBuffer,
				once:  m.state,
				index: 0,
				mask:  0,
			}

		case graphMatchOnceBuffer:
			terms := m.state.once.terms

			if m.state.index == len(terms) {
				// no matches at all or it's required all terms to be matched
				if m.state.mask == 0 || m.state.once.all {
					m.state = mismatchState
					break
				}

				// a partial match is ok
				m.state = matchState
				break
			}

			// all terms are matched
			if m.state.mask == (1<<len(terms))-1 {
				m.state = matchState
				break
			}

			for buffer := m.state; buffer.index < len(terms); buffer.index++ {
				matchFlag := 1 << buffer.index

				if buffer.mask&matchFlag == 0 {
					// IMPORTANT: else stack push must go first,
					// since it stores the state of thenStack before changes
					m.pushElseStack(buffer)
					m.pushThenStack(&matchGraphNode{
						kind: graphAddMatchOnce,
						once: buffer.once,
						mask: buffer.mask | matchFlag,
					})

					// match
					m.state = terms[buffer.index]
					buffer.index++
					break
				}
			}

		case graphAddMatchOnce:
			m.state = &matchGraphNode{
				kind:  graphMatchOnceBuffer,
				once:  m.state.once,
				index: 0,
				mask:  m.state.mask,
			}

		case graphEnum:
			if m.token != nil {
				name := toLowerCase(m.token.value)

				// drop \0 and \9 hack from keyword name
				if strings.Contains(name, `\`) {
					name = keywordHack.ReplaceAllString(name, "")
				}

				if next, ok := m.state.enumMap[name]; ok {
					m.state = next
					break
				}

				// the value with escapes decoded, see tokenNameIs
				if next, ok := enumLookup(m.state.enumMap, decodedNames(m.token)); ok {
					m.state = next
					break
				}
			}

			m.state = mismatchState

		case graphGeneric:
			var bounds *definitionsyntax.Range
			if m.syntaxStack != nil {
				bounds = m.syntaxStack.bounds
			}
			// math functions are type checked (Values 4 §10.9)
			percentSibling := m.syntaxStack != nil && m.syntaxStack.percentSibling
			rest := m.tokens[m.tokenIndex:]
			consumed, handled := matchMathFunction(m.state.generic.calc, percentSibling, rest, &m.colors, m.tokenIndex)
			if !handled {
				consumed = m.state.generic.fn(rest, bounds)
			}
			lastTokenIndex := m.tokenIndex + consumed

			if lastTokenIndex > m.tokenIndex {
				for m.tokenIndex < lastTokenIndex {
					m.addTokenToMatch()
				}

				m.state = matchState
			} else {
				m.state = mismatchState
			}

		case graphType, graphProperty:
			isType := m.state.kind == graphType
			var dictSyntax *matchGraphNode
			if m.syntaxes != nil {
				var err error
				dictSyntax, err = m.syntaxes.lookupMatch(isType, m.state.name)
				if err != nil {
					return nil, err
				}
			}

			if dictSyntax == nil {
				kind := KindProperty
				if isType {
					kind = KindType
				}
				return nil, newBadSyntaxReferenceError(kind, m.state.name)
			}

			// stash a syntax for types with low priority
			if !m.syntaxStashDisabled && m.token != nil && isType {
				// https://drafts.csswg.org/css-values-4/#custom-idents
				// When parsing positionally-ambiguous keywords in a property value, a <custom-ident> production
				// can only claim the keyword if no other unfulfilled production can claim it.
				lowPriorityCustomIdent := m.state.name == "custom-ident" && m.token.typ == tokenizer.Ident

				// https://drafts.csswg.org/css-values-4/#lengths
				// ... if a `0` could be parsed as either a <number> or a <length> in a property (such as line-height),
				// it must parse as a <number>
				lowPriorityLength := m.state.name == "length" && m.token.value == "0"

				lowPriorityMatching := lowPriorityCustomIdent || lowPriorityLength

				if lowPriorityMatching {
					if m.syntaxStash == nil {
						m.syntaxStash = m.stateSnapshotFromSyntax(m.state, m.elseStack)
					}

					m.state = mismatchState
					break
				}
			}

			m.openSyntax()
			m.state = dictSyntax

		case graphKeyword:
			name := m.state.name

			if m.token != nil {
				keywordName := m.token.value

				// drop \0 and \9 hack from keyword name
				if strings.Contains(keywordName, `\`) {
					keywordName = keywordHack.ReplaceAllString(keywordName, "")
				}

				if asciiEqualFold(keywordName, name) || tokenNameIs(m.token, name) {
					m.addTokenToMatch()
					m.state = matchState
					break
				}
			}

			m.state = mismatchState

		case graphAtKeyword, graphFunction:
			if m.token != nil && (asciiEqualFold(m.token.value, m.state.name) || tokenNameIs(m.token, m.state.name)) {
				m.addTokenToMatch()
				m.state = matchState
				break
			}

			m.state = mismatchState

		case graphToken:
			if m.token != nil && m.token.value == m.state.value {
				m.addTokenToMatch()
				m.state = matchState
				break
			}

			m.state = mismatchState

		case graphComma:
			if m.token != nil && m.token.typ == tokenizer.Comma {
				if isCommaContextStart(m.matchStack.token) {
					m.state = mismatchState
				} else {
					m.addTokenToMatch()
					if isCommaContextEnd(m.token) {
						m.state = mismatchState
					} else {
						m.state = matchState
					}
				}
			} else {
				if isCommaContextStart(m.matchStack.token) || isCommaContextEnd(m.token) {
					m.state = matchState
				} else {
					m.state = mismatchState
				}
			}

		case graphString:
			var str strings.Builder
			lastTokenIndex := m.tokenIndex

			for ; lastTokenIndex < len(m.tokens) && str.Len() < len(m.state.value); lastTokenIndex++ {
				str.WriteString(m.tokens[lastTokenIndex].value)
			}

			if asciiEqualFold(str.String(), m.state.value) {
				for m.tokenIndex < lastTokenIndex {
					m.addTokenToMatch()
				}

				m.state = matchState
			} else {
				m.state = mismatchState
			}

		default:
			return nil, &UnknownNodeTypeError{Type: m.state.kind.String()}
		}
	}

	switch reason {
	case 0:
		// upstream also logs "[csstree-match] BREAK after 15000 iterations"
		// with console.warn
		reason = exitIterationLimit
		m.matchStack = nil

	case exitMatch:
		for m.syntaxStack != nil {
			m.closeSyntax()
		}

	default:
		m.matchStack = nil
	}

	return &matchResult{
		tokens:       m.tokens,
		reason:       reason,
		iterations:   iterationCount,
		match:        m.matchStack,
		longestMatch: m.longestMatch,
	}, nil
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
