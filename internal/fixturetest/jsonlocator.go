package fixturetest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

// JSONLocator mirrors upstream's JsonLocator (lib/__tests/helpers/JsonLocator.js):
// it remembers where each top-level key of a JSON object starts and rejects
// duplicate keys at any depth.
type JSONLocator struct {
	filename string
	data     []byte
	keys     []string
	offsets  map[string]int
}

// NewJSONLocator reads the JSON object data; filename names it in locations
// and errors.
func NewJSONLocator(filename string, data []byte) (*JSONLocator, error) {
	locator := &JSONLocator{
		filename: filename,
		data:     data,
		offsets:  make(map[string]int),
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	if tok != json.Delim('{') {
		return locator, nil
	}

	for dec.More() {
		start := locator.nextTokenStart(int(dec.InputOffset()))
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filename, err)
		}
		key, _ := tok.(string)
		if _, ok := locator.offsets[key]; ok {
			return nil, fmt.Errorf("duplicate key `%s` at %s", key, locator.getLocation(start))
		}
		locator.offsets[key] = start
		locator.keys = append(locator.keys, key)

		if err := locator.checkForDuplicateKeys(dec); err != nil {
			return nil, err
		}
	}

	return locator, nil
}

// Keys returns the top-level keys in source order.
func (l *JSONLocator) Keys() []string {
	return l.keys
}

// Get returns the test name upstream builds for the top-level key name:
// "file:line:column (name)".
func (l *JSONLocator) Get(name string) (string, error) {
	offset, ok := l.offsets[name]
	if !ok {
		return "", fmt.Errorf("key `%s` not found in %s", name, l.filename)
	}
	return l.getLocation(offset) + " (" + name + ")", nil
}

// nextTokenStart skips whitespace and separators the decoder has not consumed yet.
func (l *JSONLocator) nextTokenStart(offset int) int {
	for offset < len(l.data) {
		switch l.data[offset] {
		case ' ', '\t', '\n', '\r', ',', ':':
			offset++
		default:
			return offset
		}
	}
	return offset
}

// checkForDuplicateKeys consumes one JSON value from dec.
func (l *JSONLocator) checkForDuplicateKeys(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%s: %w", l.filename, err)
	}

	switch tok {
	case json.Delim('{'):
		var keys []string
		for dec.More() {
			start := l.nextTokenStart(int(dec.InputOffset()))
			tok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("%s: %w", l.filename, err)
			}
			key, _ := tok.(string)
			if slices.Contains(keys, key) {
				return fmt.Errorf("duplicate key `%s` at %s", key, l.getLocation(start))
			}
			keys = append(keys, key)
			if err := l.checkForDuplicateKeys(dec); err != nil {
				return err
			}
		}
		return l.closeDelim(dec)
	case json.Delim('['):
		for dec.More() {
			if err := l.checkForDuplicateKeys(dec); err != nil {
				return err
			}
		}
		return l.closeDelim(dec)
	}

	return nil
}

func (l *JSONLocator) closeDelim(dec *json.Decoder) error {
	if _, err := dec.Token(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: %w", l.filename, err)
	}
	return nil
}

func (l *JSONLocator) getLocation(offset int) string {
	line := 1 + bytes.Count(l.data[:offset], []byte("\n"))
	column := 1 + offset - (bytes.LastIndexByte(l.data[:offset], '\n') + 1)
	return fmt.Sprintf("%s:%d:%d", l.filename, line, column)
}
