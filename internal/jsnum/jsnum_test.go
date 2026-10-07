package jsnum

import (
	"math"
	"strings"
	"testing"
)

// TestParse checks Parse against JavaScript's Number(). The expected values
// were produced by Node.js.
func TestParse(t *testing.T) {
	nan := math.NaN()
	negativeZero := math.Copysign(0, -1)
	for _, test := range []struct {
		input string
		want  float64
	}{
		{"", 0},
		{" ", 0},
		{"0", 0},
		{"+0", 0},
		{"-0", negativeZero},
		{"1", 1},
		{"1.", 1},
		{".5", 0.5},
		{"+.5", 0.5},
		{"-.5e3", -500},
		{"1e", nan},
		{"1e+", nan},
		{"1E-5", 1e-5},
		{"Infinity", math.Inf(1)},
		{"+Infinity", math.Inf(1)},
		{"-Infinity", math.Inf(-1)},
		{"infinity", nan},
		{"inf", nan},
		{"NaN", nan},
		{"1_000", nan},
		{"+", nan},
		{"-", nan},
		{".", nan},
		{"1.2.3", nan},
		{"00012", 12},
		{"12px", nan},
		{"0x1p3", nan},

		// whitespace and line terminators are trimmed
		{" 12 ", 12},
		{"\t3\n", 3},
		{" 1", 1},
		{"\u00a01\u2028", 1},
		{"\ufeff2", 2},
		{"\u20093", 3},

		// out of range values are ±Inf or ±0
		{"1e400", math.Inf(1)},
		{"-1e400", math.Inf(-1)},
		{"1e-400", 0},
		{"-1e-400", negativeZero},

		// 0x, 0o and 0b integers, without a sign or separators
		{"0x1F", 31},
		{"0b101", 5},
		{"0o17", 15},
		{"0XaB", 171},
		{"0B11", 3},
		{"0O17", 15},
		{"0x", nan},
		{"0b", nan},
		{"-0x10", nan},
		{"+0x1", nan},
		{"00x1", nan},
		{"0x_1", nan},
		{"0x1_0", nan},
		{"0b2", nan},
		{"0b102", nan},
		{"0o8", nan},
		// integers too large for 64 bits
		{"0x10000000000000000", 18446744073709552000},
		{"0xFFFFFFFFFFFFFFFFFF", 4.722366482869645e+21},
		{"0xfffffffffffffffff", 295147905179352830000},
		{"0b" + strings.Repeat("1", 70), 1.1805916207174113e+21},
		{"0o" + strings.Repeat("7", 30), 1.2379400392853803e+27},
		// rounded once, not digit by digit: 2^69 + 3·2^15 is nearer to
		// 2^69 + 2^17 than to 2^69
		{"0x200000000000018000", 590295810358705782784},
		{"0b1" + strings.Repeat("0", 52) + "11" + strings.Repeat("0", 20), 1.8889465931478585e+22},
	} {
		got := Parse(test.input)
		same := got == test.want && math.Signbit(got) == math.Signbit(test.want) ||
			math.IsNaN(got) && math.IsNaN(test.want)
		if !same {
			t.Errorf("Parse(%q) = %v, want %v", test.input, got, test.want)
		}
	}
}

// TestFormat checks Format against JavaScript's String(number). The
// expected values were produced by Node.js.
func TestFormat(t *testing.T) {
	for _, test := range []struct {
		value float64
		want  string
	}{
		{1e-7, "1e-7"},
		{0.000001, "0.000001"},
		{1.5, "1.5"},
		{-0.25, "-0.25"},
		{123456789012345680000, "123456789012345680000"},
		{1e21, "1e+21"},
		{1.5e21, "1.5e+21"},
		{-1e-7, "-1e-7"},
		{1<<53 + 2, "9007199254740994"},
		{1e300, "1e+300"},
		{5e-324, "5e-324"},
		{100, "100"},
		{-3, "-3"},
		{math.Copysign(0, -1), "0"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
		{math.NaN(), "NaN"},
	} {
		if got := Format(test.value); got != test.want {
			t.Errorf("Format(%v) = %q, want %q", test.value, got, test.want)
		}
	}
}
