// Ported from lib/lexer/units.js.

package csstree

// unitGroup is a group of units, the units of the dimensions of one
// type. The groups are in the order upstream's units module exports them
// (the key order of upstream's lexer.units).
type unitGroup int8

const (
	lengthUnits unitGroup = iota
	angleUnits
	timeUnits
	frequencyUnits
	resolutionUnits
	flexUnits
	decibelUnits
	semitonesUnits
	unitGroupCount
)

// unitGroupInfo is what a unit group's table entry holds.
type unitGroupInfo struct {
	// name is the group's key in the units of a lexer and the name of the
	// generic type of its dimensions.
	name string
	// base is the base type of the group's units in a calculation,
	// calcNoHint for units that a calculation does not take: CSS Grid 2
	// §7.2.4 says fr values cannot be represented in calc(), and CSS
	// Speech defines no math with <decibel> or <semitones>.
	base calcBase
}

// info returns the table entry of the unit group g.
func (g unitGroup) info() unitGroupInfo {
	return [unitGroupCount]unitGroupInfo{
		lengthUnits:     {"length", calcLength},
		angleUnits:      {"angle", calcAngle},
		timeUnits:       {"time", calcTime},
		frequencyUnits:  {"frequency", calcFrequency},
		resolutionUnits: {"resolution", calcResolution},
		flexUnits:       {"flex", calcNoHint},
		decibelUnits:    {"decibel", calcNoHint},
		semitonesUnits:  {"semitones", calcNoHint},
	}[g]
}

func (g unitGroup) name() string {
	return g.info().name
}

// defaultUnits returns the units of each unit group (upstream
// `import * as units from './units.js'`).
func defaultUnits() map[string][]string {
	return map[string][]string{
		"length": {
			// absolute length units https://www.w3.org/TR/css-values-3/#lengths
			"cm", "mm", "q", "in", "pt", "pc", "px",
			// font-relative length units https://drafts.csswg.org/css-values-4/#font-relative-lengths
			"em", "rem",
			"ex", "rex",
			"cap", "rcap",
			"ch", "rch",
			"ic", "ric",
			"lh", "rlh",
			// viewport-percentage lengths https://drafts.csswg.org/css-values-4/#viewport-relative-lengths
			"vw", "svw", "lvw", "dvw",
			"vh", "svh", "lvh", "dvh",
			"vi", "svi", "lvi", "dvi",
			"vb", "svb", "lvb", "dvb",
			"vmin", "svmin", "lvmin", "dvmin",
			"vmax", "svmax", "lvmax", "dvmax",
			// container relative lengths https://drafts.csswg.org/css-contain-3/#container-lengths
			"cqw", "cqh", "cqi", "cqb", "cqmin", "cqmax",
		},
		"angle":      {"deg", "grad", "rad", "turn"}, // https://www.w3.org/TR/css-values-3/#angles
		"time":       {"s", "ms"},                    // https://www.w3.org/TR/css-values-3/#time
		"frequency":  {"hz", "khz"},                  // https://www.w3.org/TR/css-values-3/#frequency
		"resolution": {"dpi", "dpcm", "dppx", "x"},   // https://www.w3.org/TR/css-values-3/#resolution
		"flex":       {"fr"},                         // https://drafts.csswg.org/css-grid/#fr-unit
		"decibel":    {"db"},                         // https://www.w3.org/TR/css3-speech/#mixing-props-voice-volume
		"semitones":  {"st"},                         // https://www.w3.org/TR/css3-speech/#voice-props-voice-pitch
	}
}
