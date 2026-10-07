// Go only: upstream csstree uses JavaScript's Number() and
// Number.prototype.toString directly.

// Package jsnum converts between strings and numbers as JavaScript does.
package jsnum

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Parse converts s to a number as JavaScript's Number(s) does: it trims
// whitespace, reads "" as 0, accepts decimal literals with an optional
// sign and exponent, Infinity, and 0x/0o/0b integers, and returns NaN for
// anything else. strconv.ParseFloat alone would also accept "inf", "nan",
// hexadecimal floats and underscores.
func Parse(s string) float64 {
	s = strings.TrimFunc(s, isWhiteSpace)

	if s == "" {
		return 0
	}

	switch s {
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}

	if base := integerBase(s); base != 0 {
		return parseInteger(s[2:], base)
	}

	if !isDecimalLiteral(s) {
		return math.NaN()
	}

	value, err := strconv.ParseFloat(s, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return math.NaN()
	}

	// out of range values are ±Inf or ±0, as in JavaScript
	return value
}

// integerBase returns the base of a 0x, 0o or 0b integer literal, 0 for
// other strings.
func integerBase(s string) int {
	if len(s) <= 2 || s[0] != '0' {
		return 0
	}
	switch s[1] {
	case 'x', 'X':
		return 16
	case 'o', 'O':
		return 8
	case 'b', 'B':
		return 2
	}
	return 0
}

// parseInteger returns the value of the digits of an integer literal in
// base, rounded once to the nearest float64 as JavaScript rounds the value
// of the literal, or NaN when they are not all digits of base.
func parseInteger(digits string, base int) float64 {
	for i := 0; i < len(digits); i++ {
		if value, ok := digitValue(digits[i]); !ok || value >= base {
			return math.NaN()
		}
	}
	value, ok := new(big.Int).SetString(digits, base)
	if !ok {
		return math.NaN()
	}
	f, _ := new(big.Float).SetInt(value).Float64()
	return f
}

// isWhiteSpace reports whether r is JavaScript's WhiteSpace or
// LineTerminator, which Number() trims.
func isWhiteSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x00A0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// digitValue returns the value of an ASCII digit or letter as a digit of
// a base up to 36.
func digitValue(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10, true
	}
	return 0, false
}

// isDecimalLiteral reports whether s is a StrDecimalLiteral with an
// optional sign: digits with an optional fraction, or a fraction alone,
// then an optional exponent.
func isDecimalLiteral(s string) bool {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}

	intDigits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		intDigits++
	}

	fracDigits := 0
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			fracDigits++
		}
	}

	if intDigits == 0 && fracDigits == 0 {
		return false
	}

	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		expDigits := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			expDigits++
		}
		if expDigits == 0 {
			return false
		}
	}

	return i == len(s)
}

// Format formats v as JavaScript's Number.prototype.toString does: the
// shortest digits that round-trip, written as a decimal unless the
// exponent is below -6 or above 20.
func Format(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case v == 0:
		return "0" // also for -0
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	}

	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}

	// d.ddde±x: the digits and the position n of the decimal point
	mantissa, exponent, _ := strings.Cut(strconv.FormatFloat(v, 'e', -1, 64), "e")
	e, err := strconv.Atoi(exponent)
	if err != nil {
		return sign + strconv.FormatFloat(v, 'g', -1, 64)
	}
	digits := strings.Replace(mantissa, ".", "", 1)
	k := len(digits)
	n := e + 1

	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}

	exponentSign := "+"
	if e < 0 {
		exponentSign = "-"
		e = -e
	}
	if k > 1 {
		digits = digits[:1] + "." + digits[1:]
	}
	return sign + digits + "e" + exponentSign + strconv.Itoa(e)
}
