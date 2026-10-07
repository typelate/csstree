// Ported from lib/tokenizer/OffsetToLocation.js.

package tokenizer

import "slices"

// Position is a point in a source. Offsets and columns count bytes; lines
// and columns start at 1 unless the OffsetToLocation says otherwise. Its
// JSON is upstream's {offset, line, column}.
type Position struct {
	// Offset is the byte offset from the start of the source.
	Offset int `json:"offset"`
	// Line is the line number.
	Line int `json:"line"`
	// Column is the column number, counting bytes.
	Column int `json:"column"`
}

// SourcePosition is a Position in a named source. Its JSON is upstream's
// {source, offset, line, column}.
type SourcePosition struct {
	// Filename names the source.
	Filename string `json:"source"`
	Position
}

// Location is a range in a named source, from Start up to End. Its JSON
// is upstream's node.loc: {source, start: {offset, line, column},
// end: {offset, line, column}}.
type Location struct {
	// Filename names the source.
	Filename string `json:"source"`
	// Start is the first byte of the range.
	Start Position `json:"start"`
	// End is just past the last byte of the range.
	End Position `json:"end"`
}

// OffsetToLocation converts byte offsets in Source to positions. The zero
// value converts offsets in an empty source.
//
// The line index is computed on the first lookup and again whenever Source
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

	// indexed is the Source that lineStarts describes.
	indexed  string
	computed bool
	// bomLen is the length of a leading byte order mark, which belongs to
	// no line.
	bomLen int
	// lineStarts holds the offset of the first char of every line; the
	// first line starts at bomLen.
	lineStarts []int
}

// computeLineStarts indexes the lines of Source. A line ends after an LF,
// a CR, an FF or a CRLF pair.
func (l *OffsetToLocation) computeLineStarts() {
	source := l.Source
	startOffset := bomLen(source)
	lineStarts := append(l.lineStarts[:0], startOffset)

	for i := startOffset; i < len(source); i++ {
		code := source[i]

		if code == '\n' || code == '\r' || code == '\f' {
			if code == '\r' && i+1 < len(source) && source[i+1] == '\n' {
				i++
			}

			lineStarts = append(lineStarts, i+1)
		}
	}

	l.indexed = source
	l.computed = true
	l.bomLen = startOffset
	l.lineStarts = lineStarts
}

// Position returns the position of offset, clamped to [0, len(Source)].
// Offsets inside a leading byte order mark have line and column 0.
func (l *OffsetToLocation) Position(offset int) Position {
	if !l.computed || l.indexed != l.Source {
		l.computeLineStarts()
	}

	offset = min(max(offset, 0), len(l.Source))
	if offset < l.bomLen {
		return Position{Offset: l.StartOffset + offset}
	}

	// the last line that starts at or before offset
	line, found := slices.BinarySearch(l.lineStarts, offset)
	if !found {
		line--
	}
	column := 1 + offset - l.lineStarts[line]
	if line == 0 {
		column = max(l.StartColumn, 1) + offset - l.lineStarts[0]
	}

	return Position{
		Offset: l.StartOffset + offset,
		Line:   max(l.StartLine, 1) + line,
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
