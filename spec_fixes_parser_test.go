package csstree

import (
	"encoding/json"
	"strings"
	"testing"
)

// The tests in this file cover where the parser and the generator follow
// the CSS specifications instead of upstream csstree.

// parseCollectingErrors parses source and returns the AST and the messages
// of the errors the parser recovered from.
func parseCollectingErrors(t *testing.T, source string, options ParseOptions) (Node, []string) {
	t.Helper()
	var parseErrors []string
	options.OnParseError = func(err error, _ Node) error {
		parseErrors = append(parseErrors, err.Error())
		return nil
	}
	ast, err := Parse(source, options)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", source, err)
	}
	return ast, parseErrors
}

// plainJSON returns the JSON of node without locations.
func plainJSON(t *testing.T, node Node) string {
	t.Helper()
	data, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	var plain any
	if err := json.Unmarshal(data, &plain); err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(dropLocations(plain))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func dropLocations(value any) any {
	switch value := value.(type) {
	case map[string]any:
		delete(value, "loc")
		for key, item := range value {
			value[key] = dropLocations(item)
		}
	case []any:
		for i, item := range value {
			value[i] = dropLocations(item)
		}
	}
	return value
}

// specParseTest is a source parsed in a context, with the expected plain
// object (JSON without locations; "" skips the check), node types in walk
// order ("" skips the check), generated CSS and recovered errors.
type specParseTest struct {
	source   string
	context  string
	json     string
	types    string
	generate string
	errors   string
}

func runSpecParseTests(t *testing.T, tests []specParseTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			ast, parseErrors := parseCollectingErrors(t, tt.source, ParseOptions{Context: tt.context})
			if tt.json != "" {
				if got := plainJSON(t, ast); got != tt.json {
					t.Errorf("Parse(%q) = %s, want %s", tt.source, got, tt.json)
				}
			}
			if tt.types != "" {
				if got := enterTypes(t, ast); got != tt.types {
					t.Errorf("Parse(%q) node types = %q, want %q", tt.source, got, tt.types)
				}
			}
			if got := mustGenerate(t, ast); got != tt.generate {
				t.Errorf("Generate(Parse(%q)) = %q, want %q", tt.source, got, tt.generate)
			}
			if got := strings.Join(parseErrors, "; "); got != tt.errors {
				t.Errorf("Parse(%q) errors = %q, want %q", tt.source, got, tt.errors)
			}
		})
	}
}

// Selectors 4 column combinator: https://drafts.csswg.org/selectors-4/#the-column-combinator
func TestSpecColumnCombinator(t *testing.T) {
	runSpecParseTests(t, []specParseTest{
		{
			source:   "a||b",
			context:  "selector",
			json:     `{"children":[{"name":"a","type":"TypeSelector"},{"name":"||","type":"Combinator"},{"name":"b","type":"TypeSelector"}],"type":"Selector"}`,
			generate: "a||b",
		},
		{
			source:   "a || b",
			context:  "selector",
			json:     `{"children":[{"name":"a","type":"TypeSelector"},{"name":"||","type":"Combinator"},{"name":"b","type":"TypeSelector"}],"type":"Selector"}`,
			generate: "a||b",
		},
		{
			source:   "col.selected||td",
			context:  "selector",
			types:    "Selector TypeSelector ClassSelector Combinator TypeSelector",
			generate: "col.selected||td",
		},
		{
			source:   "*||ns|b",
			context:  "selector",
			json:     `{"children":[{"name":"*","type":"TypeSelector"},{"name":"||","type":"Combinator"},{"name":"ns|b","type":"TypeSelector"}],"type":"Selector"}`,
			generate: "*||ns|b",
		},
		{
			source:   "ns|a||*|b",
			context:  "selector",
			json:     `{"children":[{"name":"ns|a","type":"TypeSelector"},{"name":"||","type":"Combinator"},{"name":"*|b","type":"TypeSelector"}],"type":"Selector"}`,
			generate: "ns|a||*|b",
		},
		{
			source:   "|| b",
			context:  "selector",
			json:     `{"children":[{"name":"||","type":"Combinator"},{"name":"b","type":"TypeSelector"}],"type":"Selector"}`,
			generate: "||b",
		},
		{
			source:   ":has(|| td){}",
			types:    "StyleSheet Rule SelectorList Selector PseudoClassSelector SelectorList Selector Combinator TypeSelector Block",
			generate: ":has(||td){}",
		},
		{
			// a namespace prefix is still read when one | follows
			source:   "a|b",
			context:  "selector",
			json:     `{"children":[{"name":"a|b","type":"TypeSelector"}],"type":"Selector"}`,
			generate: "a|b",
		},
		{
			source:   "|b",
			context:  "selector",
			json:     `{"children":[{"name":"|b","type":"TypeSelector"}],"type":"Selector"}`,
			generate: "|b",
		},
	})
}
