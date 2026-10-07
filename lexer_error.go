// Ported from lib/lexer/error.js.

package csstree

import (
	"errors"
	"regexp"
	"strings"

	"github.com/typelate/csstree/definitionsyntax"
	"github.com/typelate/csstree/tokenizer"
)

func defaultLoc() tokenizer.Position {
	return tokenizer.Position{Offset: 0, Line: 1, Column: 1}
}

// mismatchLocation is the result of locateMismatch.
type mismatchLocation struct {
	css            string
	mismatchOffset int
	mismatchLength int
	start          tokenizer.Position
	end            tokenizer.Position
}

func locateMismatch(matchResult *matchResult, node Node) mismatchLocation {
	tokens := matchResult.tokens
	longestMatch := matchResult.longestMatch
	var mismatchNode Node
	if longestMatch < len(tokens) {
		mismatchNode = tokens[longestMatch].node
	}
	var badNode Node
	if mismatchNode != node {
		badNode = mismatchNode
	}
	mismatchOffset := 0
	mismatchLength := 0
	entries := 0
	var css strings.Builder
	var start tokenizer.Position
	var end tokenizer.Position

	for i := 0; i < len(tokens); i++ {
		token := tokens[i].value

		if i == longestMatch {
			mismatchLength = len(token)
			mismatchOffset = css.Len()
		}

		if badNode != nil && tokens[i].node == badNode {
			if i <= longestMatch {
				entries++
			} else {
				entries = 0
			}
		}

		css.WriteString(token)
	}

	cssText := css.String()

	if longestMatch == len(tokens) || entries > 1 { // last
		lastNode := badNode
		if lastNode == nil {
			lastNode = node
		}
		if loc, ok := locEnd(lastNode); ok {
			start = loc
		} else {
			start = buildLoc(defaultLoc(), cssText)
		}
		end = buildLoc(start, "")
	} else {
		if loc, ok := locStart(badNode); ok {
			start = loc
		} else {
			nodeStart, ok := locStart(node)
			if !ok {
				nodeStart = defaultLoc()
			}
			start = buildLoc(nodeStart, cssText[:mismatchOffset])
		}
		if loc, ok := locEnd(badNode); ok {
			end = loc
		} else {
			end = buildLoc(start, substr(cssText, mismatchOffset, mismatchLength))
		}
	}

	return mismatchLocation{
		css:            cssText,
		mismatchOffset: mismatchOffset,
		mismatchLength: mismatchLength,
		start:          start,
		end:            end,
	}
}

// locStart returns the start of node's location; ok is false when node
// has no location (upstream fromLoc(node, 'start') returning null).
func locStart(node Node) (start tokenizer.Position, ok bool) {
	if loc := nodeLocation(node); loc != nil {
		return loc.Start, true
	}

	return tokenizer.Position{}, false
}

// locEnd is locStart for the end of node's location.
func locEnd(node Node) (end tokenizer.Position, ok bool) {
	if loc := nodeLocation(node); loc != nil {
		return loc.End, true
	}

	return tokenizer.Position{}, false
}

func nodeLocation(node Node) *Location {
	if node == nil {
		return nil
	}

	return node.Location()
}

var buildLocLineBreak = regexp.MustCompile(`\n|\r\n?|\f`)

func buildLoc(position tokenizer.Position, extra string) tokenizer.Position {
	loc := tokenizer.Position{
		Offset: position.Offset,
		Line:   position.Line,
		Column: position.Column,
	}

	if extra != "" {
		lines := buildLocLineBreak.Split(extra, -1)

		loc.Offset += len(extra)
		loc.Line += len(lines) - 1
		if len(lines) == 1 {
			loc.Column = loc.Column + len(extra)
		} else {
			loc.Column = len(lines[len(lines)-1]) + 1
		}
	}

	return loc
}

// SyntaxReferenceError reports a reference to an unknown at-rule,
// descriptor, property or type, by a Match or Check method of Lexer or by
// a syntax being matched, or a nil descriptor.
type SyntaxReferenceError struct {
	// Kind is the kind of syntax that is unknown: KindAtrule (whose
	// Reference starts with "@"), KindAtruleDescriptor, KindProperty or
	// KindType, or "" for a nil descriptor.
	Kind SyntaxKind
	// Reference is the name that is unknown ("" when there is none).
	Reference string
	// Message describes the error, such as "Unknown property `foo`" or
	// "Bad syntax reference: <foo>".
	Message string
}

// newSyntaxReferenceError returns the error of upstream
// createCustomError('SyntaxReferenceError', reason + reference).
func newSyntaxReferenceError(reason string, kind SyntaxKind, reference string) *SyntaxReferenceError {
	message := reason
	if reference != "" {
		message += " `" + reference + "`"
	}

	return &SyntaxReferenceError{
		Kind:      kind,
		Reference: reference,
		Message:   message,
	}
}

// newBadSyntaxReferenceError returns the error for a reference in a
// syntax to a type or property the lexer does not have.
func newBadSyntaxReferenceError(kind SyntaxKind, name string) *SyntaxReferenceError {
	reference := "<'" + name + "'>"
	if kind == KindType {
		reference = "<" + name + ">"
	}

	return &SyntaxReferenceError{
		Kind:      kind,
		Reference: name,
		Message:   "Bad syntax reference: " + reference,
	}
}

// Error returns Message, which is upstream's error message.
func (e *SyntaxReferenceError) Error() string {
	return e.Message
}

// ErrIterationLimit is what a *SyntaxMatchError unwraps to when the matcher
// gave up after too many iterations, which a syntax whose alternatives
// backtrack exponentially can need.
var ErrIterationLimit = errors.New("csstree: maximum match iterations exceeded")

// SyntaxMatchError reports a value that does not match a syntax. It
// unwraps to ErrIterationLimit when the matcher gave up.
//
// Offsets and columns count bytes.
type SyntaxMatchError struct {
	// Message is RawMessage followed by the syntax, the value and a caret
	// line pointing at the mismatch.
	Message string
	// RawMessage is the reason of the mismatch: "Mismatch", or "Maximum
	// iteration number exceeded (...)" when the matcher gave up.
	RawMessage string
	// Syntax is the definition syntax that was matched ("<generic>" for a
	// generic type).
	Syntax string
	// CSS is the text of the value's tokens.
	CSS string
	// MismatchOffset and MismatchLength locate the first unmatched token
	// in CSS.
	MismatchOffset int
	MismatchLength int
	// Loc is the mismatch's range in the source of the value; Filename is
	// "<unknown>" when the value has no location.
	Loc Location

	err error
}

func newSyntaxMatchError(reason exitReason, syntax definitionsyntax.Node, node Node, matchResult *matchResult) *SyntaxMatchError {
	message := reason.message()
	mismatch := locateMismatch(matchResult, node)

	err := &SyntaxMatchError{
		RawMessage:     message,
		Syntax:         "<generic>",
		CSS:            mismatch.css,
		MismatchOffset: mismatch.mismatchOffset,
		MismatchLength: mismatch.mismatchLength,
	}
	if syntax != nil {
		err.Syntax = definitionsyntax.Generate(syntax, definitionsyntax.GenerateOptions{})
	}
	value := mismatch.css
	if value == "" {
		value = "<empty string>"
	}
	err.Message = message + "\n" +
		"  syntax: " + err.Syntax + "\n" +
		"   value: " + value + "\n" +
		"  --------" + strings.Repeat("-", err.MismatchOffset) + "^"

	if reason == exitIterationLimit {
		err.err = ErrIterationLimit
	}
	source := "<unknown>"
	if node != nil {
		if loc := node.Location(); loc != nil && loc.Filename != "" {
			source = loc.Filename
		}
	}
	err.Loc = Location{
		Filename: source,
		Start:    mismatch.start,
		End:      mismatch.end,
	}

	return err
}

// Error returns Message, which is upstream's error message.
func (e *SyntaxMatchError) Error() string {
	return e.Message
}

// Unwrap returns ErrIterationLimit when the matcher gave up, else nil.
func (e *SyntaxMatchError) Unwrap() error {
	return e.err
}
