// Ported from lib/tokenizer/TokenStream.js.

package tokenizer

import "math"

const (
	offsetMask      = 1<<typeShift - 1
	typeShift       = 58
	blockOpenToken  = 1
	blockCloseToken = 2
)

// balancePair maps a block opener to its closer. It is read-only.
var balancePair = [Comment + 1]TokenType{
	Function:          RightParenthesis,
	LeftParenthesis:   RightParenthesis,
	LeftSquareBracket: RightSquareBracket,
	LeftCurlyBracket:  RightCurlyBracket,
}

// blockTokens marks block openers and closers. It is read-only.
var blockTokens = [Comment + 1]uint8{
	Function:           blockOpenToken,
	LeftParenthesis:    blockOpenToken,
	LeftSquareBracket:  blockOpenToken,
	LeftCurlyBracket:   blockOpenToken,
	RightParenthesis:   blockCloseToken,
	RightSquareBracket: blockCloseToken,
	RightCurlyBracket:  blockCloseToken,
}

// blockKind returns blockOpenToken, blockCloseToken or 0 for t.
func (t TokenType) blockKind() uint8 {
	if uint(t) < uint(len(blockTokens)) {
		return blockTokens[t]
	}
	return 0
}

// closer returns the type of the token that closes a block opened by t, or
// EOF when t opens no block.
func (t TokenType) closer() TokenType {
	if uint(t) < uint(len(balancePair)) {
		return balancePair[t]
	}
	return EOF
}

// IsBlockOpener reports whether t opens a block: a function, (, [ or {
// token.
func (t TokenType) IsBlockOpener() bool {
	return t.blockKind() == blockOpenToken
}

// IsBlockCloser reports whether t closes a block: a ), ] or } token.
func (t TokenType) IsBlockCloser() bool {
	return t.blockKind() == blockCloseToken
}

func boundIndex(index, minimum, maximum int) int {
	if index < minimum {
		return minimum
	}
	if index > maximum {
		return maximum
	}
	return index
}

// TokenStream is a cursor over the tokens of a source. NewTokenStream and
// SetSource tokenize a source and move the cursor to its first token. The
// zero value is a stream over an empty source whose cursor has not moved
// yet: it reports EOF after the first Next.
//
// The exported fields are the cursor's state, for reading only (a Parser
// embeds the stream, so syntax extensions see them): change them with
// SetSource, Reset, Next, Skip and the other methods.
//
// Tokens are stored packed as (type << 58 | end) in uint64 cells. Upstream
// packs (type << 24 | end) into uint32 cells, which silently corrupts offsets
// in sources of 16 MiB or more; 58 offset bits exceed any addressable source.
type TokenStream struct {
	// Source is the tokenized CSS text.
	Source string

	// TokenIndex, TokenType, TokenStart and TokenEnd describe the current
	// token; EOF reports that the cursor has moved past the last token.
	EOF        bool
	TokenIndex int
	TokenType  TokenType
	TokenStart int
	TokenEnd   int

	// TokenCount is the number of tokens in Source.
	TokenCount int

	// Balance holds, for every token index, the index of the matching
	// block token (or of the enclosing block's end, or TokenCount).
	Balance []int

	firstCharOffset int
	offsetAndType   []uint64
}

// NewTokenStream tokenizes source and positions the stream on the first
// token.
func NewTokenStream(source string) *TokenStream {
	ts := new(TokenStream)
	ts.SetSource(source)
	return ts
}

// Reset moves the cursor back before the first token.
func (ts *TokenStream) Reset() {
	ts.EOF = false
	ts.TokenIndex = -1
	ts.TokenType = EOF
	ts.TokenStart = ts.firstCharOffset
	ts.TokenEnd = ts.firstCharOffset
}

// SetSource tokenizes source, reusing the stream's buffers (so slices of
// Balance taken before it change), and positions the stream on the first
// token.
func (ts *TokenStream) SetSource(source string) {
	// topLevel is the balance of a token outside any block until the
	// references are reversed below (upstream uses source.length).
	const topLevel = math.MaxInt

	offsetAndType := ts.offsetAndType[:0]
	balance := ts.Balance[:0]
	firstCharOffset := -1
	balanceCloseType := EOF // no token closes the top level
	balanceStart := topLevel

	Tokenize(source, func(tokenType TokenType, start, end int) {
		index := len(offsetAndType)

		// type & offset
		offsetAndType = append(offsetAndType, uint64(tokenType)<<typeShift|uint64(end))

		if firstCharOffset == -1 {
			firstCharOffset = start
		}

		// balance
		balance = append(balance, balanceStart)

		if tokenType == balanceCloseType {
			prevBalanceStart := balance[balanceStart]

			// set reference to balance end for a block opener
			balance[balanceStart] = index

			// pop state
			balanceStart = prevBalanceStart
			balanceCloseType = EOF
			if balanceStart != topLevel {
				balanceCloseType = TokenType(offsetAndType[balanceStart] >> typeShift).closer()
			}
		} else if tokenType.IsBlockOpener() { // check for FunctionToken, <(-token>, <[-token> and <{-token>
			// push state
			balanceStart = index
			balanceCloseType = tokenType.closer()
		}
	})

	// finalize buffers
	tokenCount := len(offsetAndType)
	offsetAndType = append(offsetAndType, uint64(EOF)<<typeShift|uint64(len(source))) // <EOF-token>
	balance = append(balance, tokenCount)                                             // prevents false positive balance match with any token

	// reverse references from balance start to end
	// tokens
	//   token:   a ( [ b c ] d e ) {
	//   index:   0 1 2 3 4 5 6 7 8 9
	// before
	//   balance: 0 8 5 2 2 2 1 1 1 0
	//            - > > < < < < < < -
	// after
	//   balance: 9 8 5 5 5 2 8 8 1 9
	//            > > > > > < > > < >
	for i := range tokenCount {
		balanceStart := balance[i]

		if balanceStart <= i {
			balanceEnd := balance[balanceStart]

			if balanceEnd != i {
				balance[i] = balanceEnd
			}
		} else if balanceStart > tokenCount {
			balance[i] = tokenCount
		}
	}

	ts.Source = source
	if firstCharOffset == -1 {
		ts.firstCharOffset = 0
	} else {
		ts.firstCharOffset = firstCharOffset
	}
	ts.TokenCount = tokenCount
	ts.offsetAndType = offsetAndType
	ts.Balance = balance

	ts.Reset()
	ts.Next()
}

// packed reads offsetAndType[index], returning 0 out of range as a JavaScript
// typed array read of undefined does once masked or shifted.
func (ts *TokenStream) packed(index int) uint64 {
	if index < 0 || index >= len(ts.offsetAndType) {
		return 0
	}
	return ts.offsetAndType[index]
}

// LookupType returns the type of the token offset tokens after the current
// one, or EOF.
func (ts *TokenStream) LookupType(offset int) TokenType {
	offset += ts.TokenIndex

	if offset < ts.TokenCount {
		return TokenType(ts.packed(offset) >> typeShift)
	}

	return EOF
}

// LookupTypeNonSC returns the type of the idx-th token from the current one
// that is not whitespace or a comment, or EOF.
func (ts *TokenStream) LookupTypeNonSC(idx int) TokenType {
	for offset := ts.TokenIndex; offset < ts.TokenCount; offset++ {
		tokenType := TokenType(ts.packed(offset) >> typeShift)

		if tokenType != WhiteSpace && tokenType != Comment {
			if idx == 0 {
				return tokenType
			}
			idx--
		}
	}

	return EOF
}

// LookupOffset returns the start offset of the token offset tokens after the
// current one, or len(Source).
func (ts *TokenStream) LookupOffset(offset int) int {
	offset += ts.TokenIndex

	if offset < ts.TokenCount {
		return int(ts.packed(offset-1) & offsetMask)
	}

	return len(ts.Source)
}

// LookupOffsetNonSC returns how many tokens after the current one the idx-th
// token that is not whitespace or a comment is, or 0 when there is none.
func (ts *TokenStream) LookupOffsetNonSC(idx int) int {
	for offset := ts.TokenIndex; offset < ts.TokenCount; offset++ {
		tokenType := TokenType(ts.packed(offset) >> typeShift)

		if tokenType != WhiteSpace && tokenType != Comment {
			if idx == 0 {
				return offset - ts.TokenIndex
			}
			idx--
		}
	}

	return 0
}

// LookupValue reports whether the token offset tokens after the current one
// is an ASCII case-insensitive match for referenceStr (which must be
// lowercase).
func (ts *TokenStream) LookupValue(offset int, referenceStr string) bool {
	offset += ts.TokenIndex

	if offset < ts.TokenCount {
		return CmpStr(
			ts.Source,
			int(ts.packed(offset-1)&offsetMask),
			int(ts.packed(offset)&offsetMask),
			referenceStr,
		)
	}

	return false
}

// GetTokenStart returns the start offset of the token at tokenIndex.
func (ts *TokenStream) GetTokenStart(tokenIndex int) int {
	if tokenIndex == ts.TokenIndex {
		return ts.TokenStart
	}

	if tokenIndex > 0 {
		if tokenIndex < ts.TokenCount {
			return int(ts.packed(tokenIndex-1) & offsetMask)
		}
		return int(ts.packed(ts.TokenCount) & offsetMask)
	}

	return ts.firstCharOffset
}

// GetTokenEnd returns the end offset of the token at tokenIndex.
func (ts *TokenStream) GetTokenEnd(tokenIndex int) int {
	if tokenIndex == ts.TokenIndex {
		return ts.TokenEnd
	}

	return int(ts.packed(boundIndex(tokenIndex, 0, ts.TokenCount)) & offsetMask)
}

// GetTokenType returns the type of the token at tokenIndex.
func (ts *TokenStream) GetTokenType(tokenIndex int) TokenType {
	if tokenIndex == ts.TokenIndex {
		return ts.TokenType
	}

	return TokenType(ts.packed(boundIndex(tokenIndex, 0, ts.TokenCount)) >> typeShift)
}

// SubstrToCursor returns Source[start:TokenStart].
func (ts *TokenStream) SubstrToCursor(start int) string {
	return substring(ts.Source, start, ts.TokenStart)
}

// GetBlockTokenPairIndex returns the index of the token that closes (or
// opens) the block token at tokenIndex, or -1 when it is not a block token
// or has no pair.
func (ts *TokenStream) GetBlockTokenPairIndex(tokenIndex int) int {
	tokenType := ts.GetTokenType(tokenIndex)

	switch tokenType.blockKind() {
	case blockOpenToken:
		pairIndex, ok := ts.balanceAt(tokenIndex)
		if !ok {
			return -1 // upstream: no cell, so the pair's type is 0 and matches no closer
		}
		if tokenType.closer() == ts.GetTokenType(pairIndex) {
			return pairIndex
		}
	case blockCloseToken:
		pairIndex, ok := ts.balanceAt(tokenIndex)
		if !ok {
			return -1 // upstream: no cell, so the pair's type is 0 and opens nothing
		}
		if ts.GetTokenType(pairIndex).closer() == tokenType {
			return pairIndex
		}
	}

	return -1
}

// IsBalanceEdge reports whether the current token's balance points before
// tokenIndex.
func (ts *TokenStream) IsBalanceEdge(tokenIndex int) bool {
	balance, ok := ts.balanceAt(ts.TokenIndex)
	// upstream compares undefined (no cell, as after Reset), which is false
	return ok && balance < tokenIndex
}

// balanceAt reads Balance[index]; ok is false out of range, where a
// JavaScript typed array read gives undefined. The EOF entry at TokenCount
// is TokenCount, also in the zero value, which has no Balance.
func (ts *TokenStream) balanceAt(index int) (balance int, ok bool) {
	if index == ts.TokenCount {
		return ts.TokenCount, true
	}
	if index < 0 || index >= len(ts.Balance) {
		return 0, false
	}
	return ts.Balance[index], true
}

// IsDelim reports whether the token offset tokens after the current one (the
// current one when offset is 0) is a delim token for code.
func (ts *TokenStream) IsDelim(code, offset int) bool {
	if offset != 0 {
		return ts.LookupType(offset) == Delim &&
			CharCodeAt(ts.Source, ts.LookupOffset(offset)) == code
	}

	return ts.TokenType == Delim &&
		CharCodeAt(ts.Source, ts.TokenStart) == code
}

// Skip moves the cursor tokenCount tokens forward (or backward when
// negative), or to EOF.
func (ts *TokenStream) Skip(tokenCount int) {
	next := ts.TokenIndex + tokenCount

	if next < ts.TokenCount {
		ts.TokenIndex = next
		ts.TokenStart = int(ts.packed(next-1) & offsetMask)
		item := ts.packed(next)
		ts.TokenType = TokenType(item >> typeShift)
		ts.TokenEnd = int(item & offsetMask)
	} else {
		ts.TokenIndex = ts.TokenCount
		ts.Next()
	}
}

// Next moves the cursor to the next token, or to EOF.
func (ts *TokenStream) Next() {
	next := ts.TokenIndex + 1

	if next < ts.TokenCount {
		ts.TokenIndex = next
		ts.TokenStart = ts.TokenEnd
		item := ts.packed(next)
		ts.TokenType = TokenType(item >> typeShift)
		ts.TokenEnd = int(item & offsetMask)
	} else {
		ts.EOF = true
		ts.TokenIndex = ts.TokenCount
		ts.TokenType = EOF
		ts.TokenStart = len(ts.Source)
		ts.TokenEnd = len(ts.Source)
	}
}

// SkipSC skips whitespace and comment tokens.
func (ts *TokenStream) SkipSC() {
	for ts.TokenType == WhiteSpace || ts.TokenType == Comment {
		ts.Next()
	}
}

// StopMode is what SkipUntilBalanced does at a token, as a StopFunc decides.
type StopMode int

const (
	// NoStop goes on past the token.
	NoStop StopMode = iota
	// StopBefore stops with the token as the current one.
	StopBefore
	// StopAfter stops past the token.
	StopAfter
)

// StopFunc tells SkipUntilBalanced whether to stop at a token, given the
// char code at its start (see CharCodeAt).
type StopFunc func(code int) StopMode

// SkipUntilBalanced moves the cursor forward from startToken to the end of
// the enclosing block, stopping early where stop says so. Blocks opened on
// the way are skipped whole, without calling stop for their tokens. A nil
// stop never stops early; a negative startToken is 0.
func (ts *TokenStream) SkipUntilBalanced(startToken int, stop StopFunc) {
	if stop == nil {
		stop = func(int) StopMode { return NoStop }
	}
	cursor := max(startToken, 0)
	balanceEnd := 0
	offset := 0

loop:
	for ; cursor < ts.TokenCount; cursor++ {
		balanceEnd = ts.Balance[cursor]

		// stop scanning on balance edge that points to offset before start token
		if balanceEnd < startToken {
			break loop
		}

		if cursor > 0 {
			offset = int(ts.offsetAndType[cursor-1] & offsetMask)
		} else {
			offset = ts.firstCharOffset
		}

		// check stop condition
		switch stop(CharCodeAt(ts.Source, offset)) {
		case StopBefore:
			break loop

		case StopAfter:
			cursor++
			break loop

		default:
			// fast forward to the end of balanced block for an open block tokens
			if TokenType(ts.offsetAndType[cursor] >> typeShift).IsBlockOpener() {
				cursor = balanceEnd
			}
		}
	}

	ts.Skip(cursor - ts.TokenIndex)
}

// ForEachToken calls fn for every token in order.
func (ts *TokenStream) ForEachToken(fn func(tokenType TokenType, start, end, index int)) {
	for i, offset := 0, ts.firstCharOffset; i < ts.TokenCount; i++ {
		start := offset
		item := ts.offsetAndType[i]
		end := int(item & offsetMask)
		tokenType := TokenType(item >> typeShift)

		offset = end

		fn(tokenType, start, end, i)
	}
}

// dumpToken is one entry of dump.
type dumpToken struct {
	Index   int
	Type    string
	Chunk   string
	Balance int
}

// dump returns every token with its name, text and balance, for tests and
// debugging (upstream dump).
func (ts *TokenStream) dump() []dumpToken {
	tokens := make([]dumpToken, ts.TokenCount)

	ts.ForEachToken(func(tokenType TokenType, start, end, index int) {
		tokens[index] = dumpToken{
			Index:   index,
			Type:    tokenType.String(),
			Chunk:   substring(ts.Source, start, end),
			Balance: ts.Balance[index],
		}
	})

	return tokens
}

// substring mirrors String.prototype.substring: it clamps both ends to
// [0, len(s)] and swaps them when start > end.
func substring(s string, start, end int) string {
	start = boundIndex(start, 0, len(s))
	end = boundIndex(end, 0, len(s))
	if start > end {
		start, end = end, start
	}
	return s[start:end]
}
