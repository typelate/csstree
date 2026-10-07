// Go only: type checking of math functions, which upstream csstree
// consumes without looking at their arguments.
//
// CSS Values and Units Level 4 §10 https://drafts.csswg.org/css-values-4/#math
// CSS Typed OM Level 1 §4.3.2 (types) https://drafts.css-houdini.org/css-typed-om-1/#numeric-typing

package csstree

import (
	"strings"

	"github.com/typelate/csstree/definitionsyntax"
	"github.com/typelate/csstree/tokenizer"
)

// maxCalcDepth is the deepest nesting of parentheses and math functions a
// calculation may have. §10.8 requires at least 32 levels and makes a
// calculation with more nesting than supported invalid.
const maxCalcDepth = 256

// calcBase is a base type of a math function type (Typed OM: "length",
// "angle", "time", "frequency", "resolution" and "percent"). <flex> is
// left out: CSS Grid 2 §7.2.4 says fr values cannot be represented in
// calc(), so a flex unit in a calculation is a failure like any other
// unknown unit.
type calcBase int8

const (
	calcLength calcBase = iota
	calcAngle
	calcTime
	calcFrequency
	calcResolution
	calcPercent
	calcBaseCount

	// calcNonPercentCount is the number of base types other than percent:
	// calcPercent comes last, so range calcNonPercentCount visits the
	// others.
	calcNonPercentCount = calcPercent

	// calcNoHint is a null percent hint. As a calculation context it
	// means percentages are not allowed.
	calcNoHint calcBase = -1
)

// calcType is the type of a calculation: an exponent per base type and a
// percent hint.
type calcType struct {
	exp  [calcBaseCount]int
	hint calcBase
}

// numberCalcType is «[ ]», the type of a <number>.
func numberCalcType() calcType {
	return calcType{hint: calcNoHint}
}

// baseCalcType is «[ base → 1 ]», the type of a value of the base type.
func baseCalcType(base calcBase) calcType {
	t := numberCalcType()
	t.exp[base] = 1
	return t
}

// percentageCalcType is the type of a <percentage-token> in the
// calculation context ctx (§10.9): the type percentages resolve against
// with that type as percent hint, else «[ "percent" → 1 ]» with a percent
// hint of "percent".
func percentageCalcType(ctx calcBase) calcType {
	if ctx == calcNoHint {
		ctx = calcPercent
	}
	t := baseCalcType(ctx)
	t.hint = ctx
	return t
}

// applyPercentHint is Typed OM "apply the percent hint".
func (t calcType) applyPercentHint(hint calcBase) calcType {
	t.hint = hint
	if hint != calcPercent {
		t.exp[hint] += t.exp[calcPercent]
		t.exp[calcPercent] = 0
	}
	return t
}

// sharePercentHints is the common first step of Typed OM "add two types"
// and "multiply two types": it fails on different percent hints and
// applies one type's hint to the other.
func sharePercentHints(a, b calcType) (calcType, calcType, bool) {
	switch {
	case a.hint != calcNoHint && b.hint != calcNoHint && a.hint != b.hint:
		return a, b, false
	case a.hint != calcNoHint && b.hint == calcNoHint:
		b = b.applyPercentHint(a.hint)
	case b.hint != calcNoHint && a.hint == calcNoHint:
		a = a.applyPercentHint(b.hint)
	}
	return a, b, true
}

// addCalcTypes is Typed OM "add two types".
func addCalcTypes(a, b calcType) (calcType, bool) {
	a, b, ok := sharePercentHints(a, b)
	if !ok {
		return calcType{}, false
	}

	if a.exp == b.exp {
		return a, true
	}

	hasPercent := a.exp[calcPercent] != 0 || b.exp[calcPercent] != 0
	hasOther := a.hasNonPercentEntry() || b.hasNonPercentEntry()
	if !hasPercent || !hasOther {
		return calcType{}, false
	}

	for hint := range calcNonPercentCount {
		ha, hb := a.applyPercentHint(hint), b.applyPercentHint(hint)
		if ha.exp == hb.exp {
			return ha, true
		}
	}

	return calcType{}, false
}

// hasNonPercentEntry reports whether t has a non-zero entry other than
// percent.
func (t calcType) hasNonPercentEntry() bool {
	for base := range calcNonPercentCount {
		if t.exp[base] != 0 {
			return true
		}
	}
	return false
}

// multiplyCalcTypes is Typed OM "multiply two types".
func multiplyCalcTypes(a, b calcType) (calcType, bool) {
	a, b, ok := sharePercentHints(a, b)
	if !ok {
		return calcType{}, false
	}

	for base := range calcBaseCount {
		a.exp[base] += b.exp[base]
	}
	return a, true
}

// invert is Typed OM "invert a type".
func (t calcType) invert() calcType {
	for base := range calcBaseCount {
		t.exp[base] = -t.exp[base]
	}
	return t
}

// madeConsistentWith is Values 4 "make a type consistent" with t as the
// base.
func (t calcType) madeConsistentWith(input calcType) (calcType, bool) {
	if t.hint != calcNoHint && input.hint != calcNoHint && t.hint != input.hint {
		return calcType{}, false
	}
	if t.hint == calcNoHint {
		t.hint = input.hint
	}
	return t, true
}

// isNumber reports whether t has no non-zero entry.
func (t calcType) isNumber() bool {
	return t.exp == [calcBaseCount]int{}
}

// onlyEntry reports whether base → 1 is the only non-zero entry of t.
func (t calcType) onlyEntry(base calcBase) bool {
	for b := range calcBaseCount {
		want := 0
		if b == base {
			want = 1
		}
		if t.exp[b] != want {
			return false
		}
	}
	return true
}

// The matches methods are Typed OM "a type matches a CSS production" in
// the calculation context ctx, the type percentages resolve against
// (calcPercent when they are allowed but resolve against nothing or a
// <number>, calcNoHint when they are not allowed).

// matchesNumber reports whether t matches <number>, and so <integer>
// (§10.9).
func (t calcType) matchesNumber(ctx calcBase) bool {
	return t.isNumber() && (t.hint == calcNoHint || (ctx != calcNoHint && t.hint == ctx))
}

// matchesBase reports whether t matches the production of the single base
// type base, such as <length>, or <percentage> for calcPercent.
func (t calcType) matchesBase(base, ctx calcBase) bool {
	return t.onlyEntry(base) && (t.hint == calcNoHint || (ctx == base && t.hint == base))
}

// matchesDimension reports whether t matches <dimension>: a single base
// type other than percent.
func (t calcType) matchesDimension() bool {
	if t.hint != calcNoHint {
		return false
	}
	for base := range calcNonPercentCount {
		if t.onlyEntry(base) {
			return true
		}
	}
	return false
}

// calcWant is the kind of production a math function has to resolve to
// where it is used.
type calcWant int8

const (
	// calcWantNone is a type that takes no math function: a function is
	// left to the type's own matching.
	calcWantNone calcWant = iota
	// calcWantNothing is a type that upstream lets take math functions
	// but that no calculation resolves to (see unitGroupInfo.base).
	calcWantNothing
	calcWantNumber    // <number>, and <integer> (§10.9)
	calcWantDimension // <dimension>, a single base type other than percent
	calcWantBase      // a single base type, calcTarget.base
)

// calcTarget is how the matcher type checks a math function for a generic
// type (§10.9): what the function has to resolve to, the type its
// percentages resolve against, and the base types of the lexer's units.
// The zero value is a type that takes no math function.
type calcTarget struct {
	want calcWant
	base calcBase // for calcWantBase

	// ctx is the calculation context of the type standing alone, and
	// percentCtx when a <percentage> is an alternative of it in a "|"
	// group, as in <length-percentage> (see markPercentSiblings).
	ctx, percentCtx calcBase

	units calcUnits
}

// numberCalcTarget is the target of <number> and <integer>: percentages
// are allowed next to a <percentage>, resolving against a <number>.
func numberCalcTarget(units calcUnits) calcTarget {
	return calcTarget{want: calcWantNumber, ctx: calcNoHint, percentCtx: calcPercent, units: units}
}

// percentageCalcTarget is the target of <percentage>.
func percentageCalcTarget(units calcUnits) calcTarget {
	return calcTarget{want: calcWantBase, base: calcPercent, ctx: calcPercent, percentCtx: calcPercent, units: units}
}

// dimensionCalcTarget is the target of <dimension>, which takes no
// percentages.
func dimensionCalcTarget(units calcUnits) calcTarget {
	return calcTarget{want: calcWantDimension, ctx: calcNoHint, percentCtx: calcNoHint, units: units}
}

// unitGroupCalcTarget is the target of the generic type of the
// dimensions of a unit group. Percentages are allowed next to a
// <percentage>, resolving against the group's base type, except for
// <resolution>: there is no <resolution-percentage>.
func unitGroupCalcTarget(group unitGroup, units calcUnits) calcTarget {
	base := group.info().base
	switch base {
	case calcNoHint:
		return calcTarget{want: calcWantNothing}
	case calcResolution:
		return calcTarget{want: calcWantBase, base: base, ctx: calcNoHint, percentCtx: calcNoHint, units: units}
	}
	return calcTarget{want: calcWantBase, base: base, ctx: calcNoHint, percentCtx: base, units: units}
}

// matches reports whether a calculation of type t resolves to the
// target's production in the calculation context ctx.
func (c calcTarget) matches(t calcType, ctx calcBase) bool {
	switch c.want {
	case calcWantNumber:
		return t.matchesNumber(ctx)
	case calcWantDimension:
		return t.matchesDimension()
	case calcWantBase:
		return t.matchesBase(c.base, ctx)
	}
	return false
}

// isMathFunctionName reports whether name, a lowercased function token
// value, is a math function (§10.8, and the -moz- and -webkit- prefixed
// calc() upstream takes).
func isMathFunctionName(name string) bool {
	switch name {
	case "calc(", "-moz-calc(", "-webkit-calc(",
		"min(", "max(", "clamp(",
		"round(", "mod(", "rem(",
		"sin(", "cos(", "tan(", "asin(", "acos(", "atan(", "atan2(",
		"pow(", "sqrt(", "hypot(", "log(", "exp(",
		"abs(", "sign(":
		return true
	}
	return false
}

// isSubstitutionFunctionName reports whether name, a lowercased function
// token value, is an arbitrary substitution function (CSS Values 5 §7),
// whose value is unknown until it is substituted.
func isSubstitutionFunctionName(name string) bool {
	switch name {
	case "var(", "env(", "attr(", "if(", "inherit(":
		return true
	}
	return false
}

// isLengthFunctionName reports whether name, a lowercased function token
// value, is a non-math function that resolves to a <length> in a
// calculation: anchor() and anchor-size() (CSS Anchor Positioning 1).
func isLengthFunctionName(name string) bool {
	return name == "anchor(" || name == "anchor-size("
}

// calcUnits maps the lowercase units of a lexer's unit groups to their
// base type in a calculation.
type calcUnits map[string]calcBase

// newCalcUnits returns the base types of units, the units of a lexer's
// unit groups. A unit in more than one group has the first group's base
// type; units are compared as given, so an uppercase unit never matches.
func newCalcUnits(units map[string][]string) calcUnits {
	bases := make(calcUnits)
	for group := range unitGroupCount {
		info := group.info()
		if info.base == calcNoHint {
			continue
		}
		for _, unit := range units[info.name] {
			if _, ok := bases[unit]; !ok {
				bases[unit] = info.base
			}
		}
	}
	return bases
}

// base returns the base type of a dimension's unit, as written.
func (u calcUnits) base(unit string) (calcBase, bool) {
	if strings.Contains(unit, `\`) {
		unit = DecodeIdent(unit)
	}
	base, ok := u[asciiLowerCase(unit)]
	return base, ok
}

// matchMathFunction matches a math function at the start of tokens for a
// generic type with the calculation target. handled is false when it is
// not a math
// function or the type does not take math functions, leaving the token to
// the type's own matching. Otherwise it returns how many tokens the
// function takes, 0 when its calculation is invalid or does not resolve
// to the type (§10.9). A function containing an arbitrary substitution
// function is not checked and is consumed as upstream does.
//
// Upstream wraps the numeric generic types in math(), which consumes a
// math function without looking at its arguments; the matcher calls
// matchMathFunction before the type instead.
func matchMathFunction(target calcTarget, percentSibling bool, tokens []matchToken) (consumed int, handled bool) {
	if len(tokens) == 0 || tokens[0].typ != tokenizer.Function || target.want == calcWantNone {
		return 0, false
	}
	if !isMathFunctionName(tokenName(&tokens[0])) {
		return 0, false
	}

	length := consumeFunction(tokens)
	for i := 1; i < length; i++ {
		t := &tokens[i]
		if t.typ == tokenizer.Function && isSubstitutionFunctionName(tokenName(t)) {
			return length, true
		}
	}

	if target.want == calcWantNothing {
		return 0, true
	}

	ctx := target.ctx
	if percentSibling {
		ctx = target.percentCtx
	}
	p := &calcParser{tokens: tokens, ctx: ctx, units: target.units}
	t, ok := p.mathFunction()
	// a type matching the target also resolves to one of the §10.9
	// productions
	if !ok || !target.matches(t, ctx) {
		return 0, true
	}
	return p.pos, true
}

// calcParser parses a math function from tokens, which start with the
// function token (§10.8), and computes its type (§10.9). pos is the index
// of the token to parse next.
type calcParser struct {
	tokens []matchToken
	pos    int
	depth  int
	ctx    calcBase
	units  calcUnits
}

// skipComments advances pos past comments and returns the token there,
// nil at the end.
func (p *calcParser) skipComments() *matchToken {
	for {
		t := tokenAt(p.tokens, p.pos)
		if t == nil || t.typ != tokenizer.Comment {
			return t
		}
		p.pos++
	}
}

// skipWhiteSpace skips whitespace and comments and reports whether there
// was whitespace.
func (p *calcParser) skipWhiteSpace() bool {
	seen := false
	for {
		t := tokenAt(p.tokens, p.pos)
		if t == nil || (t.typ != tokenizer.WhiteSpace && t.typ != tokenizer.Comment) {
			return seen
		}
		seen = seen || t.typ == tokenizer.WhiteSpace
		p.pos++
	}
}

// enter counts a level of nesting; it fails past maxCalcDepth.
func (p *calcParser) enter() bool {
	p.depth++
	return p.depth <= maxCalcDepth
}

// leave counts the end of a level of nesting.
func (p *calcParser) leave() {
	p.depth--
}

// closeParenthesis consumes the ) closing a function or a parenthesized
// sum; the end of the input closes it too (CSS Syntax 3 §5.4.9).
func (p *calcParser) closeParenthesis() bool {
	p.skipWhiteSpace()
	t := p.skipComments()
	if t == nil {
		return true
	}
	if t.typ != tokenizer.RightParenthesis {
		return false
	}
	p.pos++
	return true
}

// sum is <calc-sum> = <calc-product> [ [ '+' | '-' ] <calc-product> ]*,
// with whitespace required on both sides of + and -.
func (p *calcParser) sum() (calcType, bool) {
	t, ok := p.product()
	if !ok {
		return calcType{}, false
	}

	for {
		start := p.pos
		spaceBefore := p.skipWhiteSpace()
		op := p.skipComments()
		if !isDelimToken(op, '+') && !isDelimToken(op, '-') {
			p.pos = start
			return t, true
		}
		if !spaceBefore {
			return calcType{}, false
		}
		p.pos++
		if !p.skipWhiteSpace() {
			return calcType{}, false
		}

		operand, ok := p.product()
		if !ok {
			return calcType{}, false
		}
		if t, ok = addCalcTypes(t, operand); !ok {
			return calcType{}, false
		}
	}
}

// product is <calc-product> = <calc-value> [ [ '*' | / ] <calc-value> ]*.
func (p *calcParser) product() (calcType, bool) {
	t, ok := p.value()
	if !ok {
		return calcType{}, false
	}

	for {
		start := p.pos
		p.skipWhiteSpace()
		op := p.skipComments()
		divide := isDelimToken(op, '/')
		if !divide && !isDelimToken(op, '*') {
			p.pos = start
			return t, true
		}
		p.pos++
		p.skipWhiteSpace()

		operand, ok := p.value()
		if !ok {
			return calcType{}, false
		}
		if divide {
			operand = operand.invert()
		}
		if t, ok = multiplyCalcTypes(t, operand); !ok {
			return calcType{}, false
		}
	}
}

// value is <calc-value> = <number> | <dimension> | <percentage> |
// <calc-keyword> | ( <calc-sum> ), where the numeric types include math
// functions.
func (p *calcParser) value() (calcType, bool) {
	t := p.skipComments()
	if t == nil {
		return calcType{}, false
	}

	switch t.typ {
	case tokenizer.Number:
		p.pos++
		return numberCalcType(), true

	case tokenizer.Percentage:
		p.pos++
		return percentageCalcType(p.ctx), true

	case tokenizer.Dimension:
		unit := t.value[min(tokenizer.ConsumeNumber(t.value, 0), len(t.value)):]
		base, ok := p.units.base(unit)
		if !ok {
			return calcType{}, false
		}
		p.pos++
		return baseCalcType(base), true

	case tokenizer.Ident:
		// §10.7 <calc-keyword> = e | pi | infinity | -infinity | NaN
		switch tokenName(t) {
		case "e", "pi", "infinity", "-infinity", "nan":
			p.pos++
			return numberCalcType(), true
		}
		return calcType{}, false

	case tokenizer.LeftParenthesis:
		if !p.enter() {
			return calcType{}, false
		}
		defer p.leave()
		p.pos++
		p.skipWhiteSpace()
		inner, ok := p.sum()
		if !ok || !p.closeParenthesis() {
			return calcType{}, false
		}
		return inner, true

	case tokenizer.Function:
		name := tokenName(t)
		if isMathFunctionName(name) {
			return p.mathFunction()
		}
		if isLengthFunctionName(name) {
			p.pos += consumeFunction(p.tokens[p.pos:])
			return baseCalcType(calcLength), true
		}
	}

	return calcType{}, false
}

// calcArg is an argument of a math function: a calculation, or a keyword
// standing alone (none in clamp(), a rounding strategy in round()).
type calcArg struct {
	typ     calcType
	keyword string
}

// arguments parses the comma-separated arguments of a math function up
// to and including its closing parenthesis. isKeyword tells whether a
// lowercased identifier standing alone as argument index is a keyword.
func (p *calcParser) arguments(isKeyword func(index int, keyword string) bool) ([]calcArg, bool) {
	var args []calcArg
	for {
		p.skipWhiteSpace()
		if keyword, ok := p.loneKeyword(); ok && isKeyword(len(args), keyword) {
			args = append(args, calcArg{keyword: keyword})
		} else {
			t, ok := p.sum()
			if !ok {
				return nil, false
			}
			args = append(args, calcArg{typ: t})
		}

		p.skipWhiteSpace()
		t := p.skipComments()
		switch {
		case t == nil:
			return args, true
		case t.typ == tokenizer.Comma:
			p.pos++
		case t.typ == tokenizer.RightParenthesis:
			p.pos++
			return args, true
		default:
			return nil, false
		}
	}
}

// loneKeyword consumes an identifier followed by a comma, a ) or the end
// of the input, and returns it lowercased.
func (p *calcParser) loneKeyword() (string, bool) {
	t := p.skipComments()
	if t == nil || t.typ != tokenizer.Ident {
		return "", false
	}

	start := p.pos
	p.pos++
	p.skipWhiteSpace()
	after := p.skipComments()
	if after != nil && after.typ != tokenizer.Comma && after.typ != tokenizer.RightParenthesis {
		p.pos = start
		return "", false
	}
	p.pos = start + 1
	return tokenName(t), true
}

// noKeywords is the isKeyword of arguments for functions without
// keyword arguments.
func noKeywords(int, string) bool {
	return false
}

// sumOf adds the types of calculations (§10.9 for min(), max(), clamp(),
// hypot(), round(), mod(), rem()).
func sumOf(args []calcArg) (calcType, bool) {
	var sum calcType
	first := true
	for _, arg := range args {
		if arg.keyword != "" {
			continue
		}
		if first {
			sum, first = arg.typ, false
			continue
		}
		var ok bool
		if sum, ok = addCalcTypes(sum, arg.typ); !ok {
			return calcType{}, false
		}
	}
	return sum, !first
}

// mathFunction parses the math function at pos (§10.1-§10.6) and returns
// its type (§10.9).
func (p *calcParser) mathFunction() (calcType, bool) {
	if !p.enter() {
		return calcType{}, false
	}
	defer p.leave()

	name := tokenName(p.skipComments())
	p.pos++

	switch name {
	case "calc(", "-moz-calc(", "-webkit-calc(", "abs(":
		return p.singleArgument()

	case "min(", "max(", "hypot(":
		args, ok := p.arguments(noKeywords)
		if !ok {
			return calcType{}, false
		}
		return sumOf(args)

	case "clamp(":
		// clamp( [ <calc-sum> | none ], <calc-sum>, [ <calc-sum> | none ] )
		args, ok := p.arguments(func(index int, keyword string) bool {
			return index != 1 && keyword == "none"
		})
		if !ok || len(args) != 3 {
			return calcType{}, false
		}
		return sumOf(args)

	case "round(":
		return p.round()

	case "mod(", "rem(":
		return p.sumOfArguments(2)

	case "sin(", "cos(", "tan(":
		// a <number> or an <angle>, returning a <number>
		arg, ok := p.singleArgument()
		if !ok || (!arg.matchesNumber(p.ctx) && !arg.matchesBase(calcAngle, p.ctx)) {
			return calcType{}, false
		}
		return numberCalcType().madeConsistentWith(arg)

	case "asin(", "acos(", "atan(":
		// a <number>, returning an <angle>
		arg, ok := p.singleArgument()
		if !ok || !arg.matchesNumber(p.ctx) {
			return calcType{}, false
		}
		return baseCalcType(calcAngle).madeConsistentWith(arg)

	case "atan2(":
		sum, ok := p.sumOfArguments(2)
		if !ok {
			return calcType{}, false
		}
		return baseCalcType(calcAngle).madeConsistentWith(sum)

	case "pow(", "log(":
		// pow(A, B), log(A, B?): <number>s of a consistent type
		args, ok := p.arguments(noKeywords)
		if !ok || len(args) > 2 || (name == "pow(" && len(args) != 2) {
			return calcType{}, false
		}
		for _, arg := range args {
			if !arg.typ.matchesNumber(p.ctx) {
				return calcType{}, false
			}
		}
		sum, ok := sumOf(args)
		if !ok {
			return calcType{}, false
		}
		return numberCalcType().madeConsistentWith(sum)

	case "sqrt(", "exp(":
		arg, ok := p.singleArgument()
		if !ok || !arg.matchesNumber(p.ctx) {
			return calcType{}, false
		}
		return numberCalcType().madeConsistentWith(arg)

	case "sign(":
		arg, ok := p.singleArgument()
		if !ok {
			return calcType{}, false
		}
		return numberCalcType().madeConsistentWith(arg)
	}

	return calcType{}, false
}

// sumOfArguments parses exactly n arguments, none of them a keyword, and
// returns the sum of their types.
func (p *calcParser) sumOfArguments(n int) (calcType, bool) {
	args, ok := p.arguments(noKeywords)
	if !ok || len(args) != n {
		return calcType{}, false
	}
	return sumOf(args)
}

// singleArgument parses the one argument, not a keyword, of a math
// function and returns its type.
func (p *calcParser) singleArgument() (calcType, bool) {
	return p.sumOfArguments(1)
}

// round parses round( <rounding-strategy>?, <calc-sum>, <calc-sum>? )
// (§10.3).
func (p *calcParser) round() (calcType, bool) {
	args, ok := p.arguments(func(index int, keyword string) bool {
		if index != 0 {
			return false
		}
		switch keyword {
		case "nearest", "up", "down", "to-zero", "line-width":
			return true
		}
		return false
	})
	if !ok {
		return calcType{}, false
	}

	strategy := "nearest"
	if len(args) > 0 && args[0].keyword != "" {
		strategy = args[0].keyword
		args = args[1:]
	}
	if len(args) == 0 || len(args) > 2 {
		return calcType{}, false
	}

	a := args[0].typ
	if strategy == "line-width" && !a.matchesBase(calcLength, p.ctx) {
		return calcType{}, false
	}
	// B may be omitted when A is a <number> or for line-width
	if len(args) == 1 && strategy != "line-width" && !a.matchesNumber(p.ctx) {
		return calcType{}, false
	}
	return sumOf(args)
}

// markPercentSiblings marks the Type terms of a "|" group that also has a
// <percentage> term, so that a math function matched for a numeric type
// among them resolves percentages against that type (§10.9.1 calculation
// contexts; see calcTarget.percentCtx).
func markPercentSiblings(group *definitionsyntax.Group, terms []*matchGraphNode) {
	if group.Combinator != definitionsyntax.SingleBar {
		return
	}

	hasPercentage := false
	for _, term := range group.Terms {
		if t, ok := term.(*definitionsyntax.Type); ok && t.Name == "percentage" {
			hasPercentage = true
			break
		}
	}
	if !hasPercentage {
		return
	}

	for _, term := range terms {
		if term.typ == "Type" {
			term.percentSibling = true
		}
	}
}
