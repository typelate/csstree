// Ported from lib/tokenizer/OffsetToLocation.js.

package tokenizer

import "unicode/utf8"

// Position is a point in a source: upstream's {offset, line, column}.
// Offsets and columns count bytes; lines and columns start at 1 unless the
// OffsetToLocation says otherwise.
type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

// SourcePosition is a Position in a named source: upstream's
// {source, offset, line, column}.
type SourcePosition struct {
	Filename string `json:"source"`
	Position
}

// Location is a range in a named source, upstream's node.loc:
// {source, start: {offset, line, column}, end: {offset, line, column}}.
type Location struct {
	Filename string   `json:"source"`
	Start    Position `json:"start"`
	End      Position `json:"end"`
}

// OffsetToLocation converts byte offsets in Source to positions. The zero
// value converts offsets in an empty source.
//
// The line table is computed on the first lookup and again whenever Source
// has changed since, so an OffsetToLocation can be reused for several
// sources by setting its fields.
type OffsetToLocation struct {
	// Source is the CSS text the offsets point into.
	Source string
	// Filename names the source in every Location.
	Filename string
	// StartOffset is added to every offset; StartLine and StartColumn are
	// the line and column of the first char of Source. Zero StartLine and
	// StartColumn mean 1.
	StartOffset int
	StartLine   int
	StartColumn int

	// indexed is the Source that lines describes.
	indexed  string
	computed bool
	// bomLen is the length of a leading byte order mark, which belongs to
	// no line.
	bomLen int
	// lines holds, for every offset from bomLen to len(indexed), the line
	// (counted from 0) and the column in it (counted from 0).
	lines []lineColumn
}

type lineColumn struct {
	line, column int
}

const (
	lf = '\n' // upstream N
	ff = '\f' // upstream F
	cr = '\r' // upstream R
)

func (l *OffsetToLocation) computeLinesAndColumns() {
	source := l.Source
	first, _ := utf8.DecodeRuneInString(source)
	startOffset := IsBOM(int(first))
	lines := l.lines[:0]
	line := 0
	column := 0

	for i := startOffset; i < len(source); i++ {
		code := source[i]

		lines = append(lines, lineColumn{line, column})
		column++

		if code == lf || code == cr || code == ff {
			if code == cr && i+1 < len(source) && source[i+1] == lf {
				i++
				lines = append(lines, lineColumn{line, column})
			}

			line++
			column = 0
		}
	}

	lines = append(lines, lineColumn{line, column})

	l.indexed = source
	l.computed = true
	l.bomLen = startOffset
	l.lines = lines
}

// Position returns the position of offset, clamped to [0, len(Source)].
// Offsets inside a leading byte order mark have line and column 0, as
// upstream's tables leave them.
func (l *OffsetToLocation) Position(offset int) Position {
	if !l.computed || l.indexed != l.Source {
		l.computeLinesAndColumns()
	}

	offset = min(max(offset, 0), len(l.Source))
	if offset < l.bomLen {
		return Position{Offset: l.StartOffset + offset}
	}

	lc := l.lines[offset-l.bomLen]
	line := max(l.StartLine, 1) + lc.line
	column := 1 + lc.column
	if lc.line == 0 {
		column = max(l.StartColumn, 1) + lc.column
	}

	return Position{
		Offset: l.StartOffset + offset,
		Line:   line,
		Column: column,
	}
}

// Range returns the location of [start, end) in Filename; both ends are
// clamped as by Position.
func (l *OffsetToLocation) Range(start, end int) Location {
	return Location{
		Filename: l.Filename,
		Start:    l.Position(start),
		End:      l.Position(end),
	}
}
