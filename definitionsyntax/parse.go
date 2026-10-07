// Ported from lib/definition-syntax/parse.js.

package definitionsyntax

import (
	"maps"
	"slices"
	"strconv"
)

const (
	exclamationMark    = 33  // !
	numberSign         = 35  // #
	ampersand          = 38  // &
	apostrophe         = 39  // '
	leftParenthesis    = 40  // (
	rightParenthesis   = 41  // )
	asterisk           = 42  // *
	plusSign           = 43  // +
	comma              = 44  // ,
	hyphenMinus        = 45  // -
	lessThanSign       = 60  // <
	greaterThanSign    = 62  // >
	questionMark       = 63  // ?
	commercialAt       = 64  // @
	leftSquareBracket  = 91  // [
	rightSquareBracket = 93  // ]
	leftCurlyBracket   = 123 // {
	verticalLine       = 124 // |
	rightCurlyBracket  = 125 // }
	infinity           = "∞" // U+221E, more than one byte in a Go string
	minusSign          = "−" // U+2212, as CSS Values §2.4.1 writes −∞
)

// precedence is upstream COMBINATOR_PRECEDENCE: a lower value binds
// tighter.
func (c Combinator) precedence() int {
	switch c {
	case Juxtaposition:
		return 1
	case DoubleAmpersand:
		return 2
	case DoubleBar:
		return 3
	case SingleBar:
		return 4
	}
	return 0
}

// combinatorNode and spacesNode are the transient { type: 'Combinator' } and
// { type: 'Spaces' } tokens; they never appear in a parsed AST.
type combinatorNode struct {
	value Combinator
}

type spacesNode struct {
	value string
}

func (*combinatorNode) Type() string { return "Combinator" }
func (*spacesNode) Type() string     { return "Spaces" }
func (*combinatorNode) node()        {}
func (*spacesNode) node()            {}

func isCombinator(node Node) bool {
	_, ok := node.(*combinatorNode)
	return ok
}

// scanInt reads a number for a multiplier range.
func (s *scanner) scanInt() int {
	start := s.pos
	digits := s.scanNumber()
	value, err := strconv.Atoi(digits)
	if err != nil {
		// digits are 0-9 only, so the number is too large for an int;
		// upstream would keep it as an inexact float
		s.pos = start
		s.error("Number is out of range")
	}
	return value
}

// scanFloat reads a number for a bracketed range; like Number(), it gives
// ±Inf for a number too large for a float64.
func (s *scanner) scanFloat() float64 {
	value, err := strconv.ParseFloat(s.scanNumber(), 64)
	if err != nil {
		// digits are 0-9 only, so the only possible error is ErrRange,
		// for which value is already ±Inf as Number() returns
		return value
	}
	return value
}

func (s *scanner) readMultiplierRange() (min, max int) {
	s.eat(leftCurlyBracket)
	s.skipWs()

	min = s.scanInt()
	s.skipWs()

	if s.charCode() == comma {
		s.pos++
		s.skipWs()

		max = Unbounded
		if s.charCode() != rightCurlyBracket {
			// {A,B}: "at least A and at most B"; upstream accepts B < A
			maxStart := s.pos
			max = s.scanInt()
			if max < min {
				s.pos = maxStart
				s.error("Wrong multiplier range")
			}
			s.skipWs()
		}
	} else {
		max = min
	}

	s.eat(rightCurlyBracket)

	return min, max
}

func (s *scanner) readMultiplier() *Multiplier {
	var min, max int
	isComma := false

	switch s.charCode() {
	case asterisk:
		s.pos++

		min, max = 0, Unbounded

	case plusSign:
		s.pos++

		min, max = 1, Unbounded

	case questionMark:
		s.pos++

		min, max = 0, 1

	case numberSign:
		s.pos++

		isComma = true

		if s.charCode() == leftCurlyBracket {
			min, max = s.readMultiplierRange()
		} else if s.charCode() == questionMark {
			// https://www.w3.org/TR/css-values-4/#component-multipliers
			// > the # and ? multipliers may be stacked as #?
			// In this case just treat "#?" as a single multiplier
			// { min: 0, max: 0, comma: true }
			s.pos++
			min, max = 0, Unbounded
		} else {
			min, max = 1, Unbounded
		}

	case leftCurlyBracket:
		min, max = s.readMultiplierRange()

	default:
		return nil
	}

	return &Multiplier{
		Comma: isComma,
		Min:   min,
		Max:   max,
		Term:  nil,
	}
}

func (s *scanner) maybeMultiplied(node Node) Node {
	multiplier := s.readMultiplier()

	if multiplier != nil {
		multiplier.Term = node

		// https://www.w3.org/TR/css-values-4/#component-multipliers
		// > The + and # multipliers may be stacked as +#;
		// Represent "+#" as nested multipliers:
		// { ...<multiplier #>,
		//   term: {
		//     ...<multipler +>,
		//     term: node
		//   }
		// }
		if s.charCode() == numberSign &&
			s.charCodeAt(s.pos-1) == plusSign {
			return s.maybeMultiplied(multiplier)
		}

		// https://www.w3.org/TR/css-values-4/#component-multipliers
		// > the # and ? multipliers, {A} and ? multipliers, and {A,B} and ? multipliers
		// > may be stacked as #?, {A}?, and {A,B}?, respectively
		// Represent "{}?" as nested multipliers as well as "+#".
		// The "#?" case is already handled above, in maybeMultiplied()
		if s.charCode() == questionMark &&
			s.charCodeAt(s.pos-1) == rightCurlyBracket {
			return s.maybeMultiplied(multiplier)
		}

		return multiplier
	}

	return node
}

func (s *scanner) maybeToken() Node {
	ch := s.peek()

	if ch == "" {
		return nil
	}

	return s.maybeMultiplied(&Token{
		Value: ch,
	})
}

func (s *scanner) readProperty() Node {
	s.eat(lessThanSign)
	s.eat(apostrophe)

	name := s.scanWord()

	s.eat(apostrophe)
	s.eat(greaterThanSign)

	return s.maybeMultiplied(&Property{
		Name: name,
	})
}

// https://drafts.csswg.org/css-values-3/#numeric-ranges
// 4.1. Range Restrictions and Range Definition Notation
//
// Range restrictions can be annotated in the numeric type notation using CSS bracketed
// range notation—[min,max]—within the angle brackets, after the identifying keyword,
// indicating a closed range between (and including) min and max.
// For example, <integer [0, 10]> indicates an integer between 0 and 10, inclusive.
func (s *scanner) readTypeRange() *Range {
	// use nil for Infinity to make AST format JSON serializable/deserializable
	var min *RangeBound // -Infinity
	var max *RangeBound // Infinity
	sign := 1.0

	s.eat(leftSquareBracket)

	sign = s.readSign()

	if sign == -1 && s.hasPrefix(infinity) {
		s.peek()
	} else {
		min = &RangeBound{Value: sign * s.scanFloat()}

		if s.isNameCharCode(s.charCode()) {
			min.Unit = s.scanWord()
		}
	}

	s.skipWs()
	s.eat(comma)
	s.skipWs()

	if s.hasPrefix(infinity) {
		s.peek()
	} else {
		sign = s.readSign()

		max = &RangeBound{Value: sign * s.scanFloat()}

		if s.isNameCharCode(s.charCode()) {
			max.Unit = s.scanWord()
		}
	}

	s.eat(rightSquareBracket)

	return &Range{
		Min: min,
		Max: max,
	}
}

// readSign moves past a U+002D HYPHEN-MINUS or a U+2212 MINUS SIGN and
// returns -1, or returns 1 when there is neither (upstream accepts only the
// hyphen-minus).
func (s *scanner) readSign() float64 {
	if s.charCode() == hyphenMinus {
		s.pos++
		return -1
	}
	if s.hasPrefix(minusSign) {
		s.pos += len(minusSign)
		return -1
	}
	return 1
}

func (s *scanner) readType() Node {
	var opts *Range

	s.eat(lessThanSign)
	name := s.scanWord()

	// https://drafts.csswg.org/css-values-5/#boolean
	if name == "boolean-expr" {
		s.eat(leftSquareBracket)

		implicitGroup := s.readImplicitGroup(rightSquareBracket)

		s.eat(rightSquareBracket)
		s.eat(greaterThanSign)

		var term Node = implicitGroup
		if len(implicitGroup.Terms) == 1 {
			term = implicitGroup.Terms[0]
		}

		return s.maybeMultiplied(&Boolean{
			Term: term,
		})
	}

	if s.charCode() == leftParenthesis &&
		s.nextCharCode() == rightParenthesis {
		s.pos += 2
		name += "()"
	}

	if s.charCodeAt(s.findWsEnd(s.pos)) == leftSquareBracket {
		s.skipWs()
		opts = s.readTypeRange()
	}

	s.eat(greaterThanSign)

	return s.maybeMultiplied(&Type{
		Name: name,
		Opts: opts,
	})
}

func (s *scanner) readKeywordOrFunction() Node {
	name := s.scanWord()

	if s.charCode() == leftParenthesis {
		s.pos++

		return &Function{
			Name: name,
		}
	}

	return s.maybeMultiplied(&Keyword{
		Name: name,
	})
}

// regroupTerms returns the regrouped terms, since a Go slice cannot be
// spliced in place, and the lowest precedence combinator ("" for none).
func regroupTerms(terms []Node, combinators map[Combinator]bool) ([]Node, Combinator) {
	createGroup := func(terms []Node, combinator Combinator) *Group {
		return &Group{
			Terms:         terms,
			Combinator:    combinator,
			DisallowEmpty: false,
			Explicit:      false,
		}
	}

	var combinator Combinator

	sorted := slices.SortedFunc(maps.Keys(combinators), func(a, b Combinator) int {
		return a.precedence() - b.precedence()
	})

	for len(sorted) > 0 {
		combinator = sorted[0]
		sorted = sorted[1:]

		i := 0
		subgroupStart := 0

		for ; i < len(terms); i++ {
			term, ok := terms[i].(*combinatorNode)

			if ok {
				if term.value == combinator {
					if subgroupStart == -1 {
						subgroupStart = i - 1
					}
					terms = slices.Delete(terms, i, i+1)
					i--
				} else {
					if subgroupStart != -1 && i-subgroupStart > 1 {
						terms = slices.Replace(terms,
							subgroupStart,
							i,
							Node(createGroup(slices.Clone(terms[subgroupStart:i]), combinator)),
						)
						i = subgroupStart + 1
					}
					subgroupStart = -1
				}
			}
		}

		if subgroupStart != -1 && len(sorted) > 0 {
			terms = slices.Replace(terms,
				subgroupStart,
				i,
				Node(createGroup(slices.Clone(terms[subgroupStart:i]), combinator)),
			)
		}
	}

	return terms, combinator
}

// readImplicitGroup reads the terms up to the first stop char outside a
// nested group, which it leaves unread, or to the end of the source.
func (s *scanner) readImplicitGroup(stop byte) *Group {
	return s.readTerms(func() bool { return s.charCode() == stop })
}

// readSourceGroup reads the terms of the whole source: upstream's
// readImplicitGroup without a stop char code.
func (s *scanner) readSourceGroup() *Group {
	return s.readTerms(func() bool { return false })
}

// readTerms reads terms until atStop reports true or there is no term left,
// and groups them by their combinators.
func (s *scanner) readTerms(atStop func() bool) *Group {
	combinators := make(map[Combinator]bool)
	terms := []Node{}
	var prevToken Node
	prevTokenPos := s.pos
	prevTokenIsFunction := false

	for !atStop() {
		var token Node
		if prevTokenIsFunction {
			token = s.readImplicitGroup(rightParenthesis)
		} else {
			token = s.readTerm()
		}

		if token == nil {
			break
		}

		if _, ok := token.(*spacesNode); ok {
			continue
		}

		if prevTokenIsFunction {
			group := token.(*Group)

			if len(group.Terms) == 0 {
				prevTokenIsFunction = false
				continue
			}

			if group.Combinator == Juxtaposition {
				for len(group.Terms) > 1 {
					combinators[Juxtaposition] = true // a b
					terms = append(terms, &combinatorNode{
						value: Juxtaposition,
					}, group.Terms[0])
					group.Terms = group.Terms[1:]
				}

				token = group.Terms[0]
			}
		}

		if combinator, ok := token.(*combinatorNode); ok {
			// check for combinator in group beginning and double combinator sequence
			if prevToken == nil || isCombinator(prevToken) {
				s.pos = prevTokenPos
				s.error("Unexpected combinator")
			}

			combinators[combinator.value] = true
		} else if prevToken != nil && !isCombinator(prevToken) {
			combinators[Juxtaposition] = true // a b
			terms = append(terms, &combinatorNode{
				value: Juxtaposition,
			})
		}

		terms = append(terms, token)
		prevToken = token
		prevTokenPos = s.pos
		_, prevTokenIsFunction = token.(*Function)
	}

	// check for combinator in group ending
	if prevToken != nil && isCombinator(prevToken) {
		// Upstream subtracts (this.pos -= prevTokenPos) where it surely
		// means to assign: the offset becomes the number of bytes read after
		// the trailing combinator, usually 0, which the error message shows
		// as the end of the input. The port keeps the quirk, since the error
		// fixtures compare the offset and message.
		s.pos -= prevTokenPos
		s.error("Unexpected combinator")
	}

	terms, combinator := regroupTerms(terms, combinators)
	if combinator == "" {
		combinator = Juxtaposition
	}

	return &Group{
		Terms:         terms,
		Combinator:    combinator,
		DisallowEmpty: false,
		Explicit:      false,
	}
}

func (s *scanner) readGroup() *Group {
	s.eat(leftSquareBracket)
	result := s.readImplicitGroup(rightSquareBracket)
	s.eat(rightSquareBracket)

	result.Explicit = true

	if s.charCode() == exclamationMark {
		s.pos++
		result.DisallowEmpty = true
	}

	return result
}

// readTerm returns nil where upstream returns undefined.
func (s *scanner) readTerm() Node {
	code := s.charCode()

	switch code {
	case rightSquareBracket:
		// don't eat, stop scan a group

	case leftSquareBracket:
		return s.maybeMultiplied(s.readGroup())

	case lessThanSign:
		if s.nextCharCode() == apostrophe {
			return s.readProperty()
		}
		return s.readType()

	case verticalLine:
		end := s.pos + 1
		if s.nextCharCode() == verticalLine {
			end = s.pos + 2
		}
		return &combinatorNode{
			value: Combinator(s.substringToPos(end)),
		}

	case ampersand:
		s.pos++
		s.eat(ampersand)

		return &combinatorNode{
			value: DoubleAmpersand,
		}

	case comma:
		s.pos++
		return &Comma{}

	case apostrophe:
		return s.maybeMultiplied(&String{
			Value: s.scanString(),
		})

	case space, tab, lineFeed, carriageReturn, formFeed:
		return &spacesNode{
			value: s.scanSpaces(),
		}

	case commercialAt:
		code = s.nextCharCode()

		if s.isNameCharCode(code) {
			s.pos++
			return &AtKeyword{
				Name: s.scanWord(),
			}
		}

		return s.maybeToken()

	case asterisk, plusSign, questionMark, numberSign, exclamationMark:
		// prohibited tokens (used as a multiplier start)

	case leftCurlyBracket:
		// LEFTCURLYBRACKET is allowed since mdn/data uses it w/o quoting
		// check next char isn't a number, because it's likely a disjoined multiplier
		code = s.nextCharCode()

		if code < 48 || code > 57 {
			return s.maybeToken()
		}

	default:
		if s.isNameCharCode(code) {
			return s.readKeywordOrFunction()
		}

		return s.maybeToken()
	}

	return nil
}

// Parse parses a value definition syntax. The result is always a Group. A
// malformed source gives a *SyntaxError.
func Parse(source string) (result *Group, err error) {
	defer func() {
		if r := recover(); r != nil {
			syntaxError, ok := r.(*SyntaxError)
			if !ok {
				panic(r)
			}
			result, err = nil, syntaxError
		}
	}()

	s := newScanner(source)
	result = s.readSourceGroup()

	if s.pos != len(source) {
		s.error("Unexpected input")
	}

	// reduce redundant groups with single group term
	if len(result.Terms) == 1 {
		if group, ok := result.Terms[0].(*Group); ok {
			return group, nil
		}
	}

	return result, nil
}
