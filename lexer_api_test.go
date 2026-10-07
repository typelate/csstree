package csstree

import (
	"encoding/json"
	"errors"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/typelate/csstree/definitionsyntax"
)

// The expectations in this file are upstream csstree 3.2.1's results under
// Node.js.

// TestNewLexerCustomUnits checks a lexer whose config replaces unit
// groups: a list replaces the group (units compare lowercased against the
// list as given), nil keeps the default and an empty list leaves the group
// without units. <attr-unit> takes every unit lowercased.
func TestNewLexerCustomUnits(t *testing.T) {
	lexer, err := DefaultSyntax().NewLexer(&Config{
		Generic: new(true),
		Units: map[string][]string{
			"length": {"foo", "BAR"},
			"angle":  nil,
			"time":   {},
		},
		Types: map[string]string{"a": "<length>"},
	})
	if err != nil {
		t.Fatal(err)
	}

	runMatchCases(t, lexer, []matchCase{
		{kind: "type", name: "length", value: "1foo", matched: true},
		{kind: "type", name: "length", value: "1FOO", matched: true},
		{kind: "type", name: "length", value: "1bar"},
		{kind: "type", name: "length", value: "1px"},
		{kind: "type", name: "length", value: "0", matched: true},
		{kind: "type", name: "length", value: "1deg"},
		{kind: "type", name: "a", value: "1foo", matched: true},
		{kind: "type", name: "angle", value: "1deg", matched: true},
		{kind: "type", name: "time", value: "1s"},
		{kind: "type", name: "time", value: "1ms"},
		{kind: "syntax", name: "<attr-unit>", value: "foo", matched: true},
		{kind: "syntax", name: "<attr-unit>", value: "BAR", matched: true},
		{kind: "syntax", name: "<attr-unit>", value: "px"},
		{kind: "syntax", name: "<attr-unit>", value: "deg", matched: true},
		{kind: "syntax", name: "<attr-unit>", value: "%", matched: true},
		{kind: "syntax", name: "<attr-unit>", value: "s"},
	})

	want := defaultUnits()
	want["length"] = []string{"foo", "BAR"}
	want["time"] = []string{}
	if got := lexer.Units(); !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("Units() = %v, want %v", got, want)
	}
}

func TestLexerAccessors(t *testing.T) {
	lexer := DefaultLexer()

	t.Run("CSSWideKeywords", func(t *testing.T) {
		want := []string{"initial", "inherit", "unset", "revert", "revert-layer"}
		if got := lexer.CSSWideKeywords(); !slices.Equal(got, want) {
			t.Errorf("CSSWideKeywords() = %v, want %v", got, want)
		}
	})

	t.Run("StructureDocs", func(t *testing.T) {
		for nodeType, want := range map[string]map[string]string{
			"Dimension":   {"type": `"Dimension"`, "value": "string", "unit": "string"},
			"Declaration": {"type": `"Declaration"`, "important": "boolean | string", "property": "string", "value": "Value | Raw"},
			"Nope":        nil,
		} {
			if got := lexer.StructureDocs(nodeType); !maps.Equal(got, want) {
				t.Errorf("StructureDocs(%q) = %v, want %v", nodeType, got, want)
			}
		}
	})

	t.Run("AtrulePrelude", func(t *testing.T) {
		for _, test := range []struct {
			name string
			want bool
		}{
			{"media", true},
			{"MEDIA", true},
			{"font-face", false},
			{"nope", false},
			{"-webkit-media", true},
		} {
			prelude := lexer.AtrulePrelude(test.name)
			if got := prelude != nil; got != test.want {
				t.Errorf("AtrulePrelude(%q) = %v, want a prelude: %v", test.name, prelude, test.want)
			}
			if prelude != nil && (prelude.Kind() != KindAtrulePrelude || prelude.Name() != "media") {
				t.Errorf("AtrulePrelude(%q) = %s %s, want AtrulePrelude media", test.name, prelude.Kind(), prelude.Name())
			}
		}
	})

	t.Run("Property and Atrule", func(t *testing.T) {
		for _, test := range []struct {
			name, want string
		}{
			{"color", "color"},
			{"COLOR", "color"},
			{"-webkit-color", "color"},
			{"_color", "color"},
			{"-webkit-box-reflect", "-webkit-box-reflect"},
			{"nope", ""},
		} {
			got := ""
			if property := lexer.Property(test.name); property != nil {
				got = property.Name()
			}
			if got != test.want {
				t.Errorf("Property(%q) = %q, want %q", test.name, got, test.want)
			}
		}
		if atrule := lexer.Atrule("-moz-Font-Face"); atrule == nil || atrule.Name() != "font-face" || atrule.Descriptor("src") == nil {
			t.Errorf("Atrule(-moz-Font-Face) = %v, want font-face with src", atrule)
		}
		if atrule := lexer.Atrule("nope"); atrule != nil {
			t.Errorf("Atrule(nope) = %v, want nil", atrule)
		}
	})

	t.Run("Dump", func(t *testing.T) {
		dump, err := lexer.Dump()
		if err != nil {
			t.Fatal(err)
		}
		media := dump.Atrules["media"]
		if media.Prelude != "<media-query-list>" || media.Descriptors != nil {
			t.Errorf("Dump().Atrules[media] = %+v, want prelude <media-query-list> and no descriptors", media)
		}
		fontFace := dump.Atrules["font-face"]
		if fontFace.Prelude != "" || len(fontFace.Descriptors) != 13 {
			t.Errorf("Dump().Atrules[font-face] = %+v, want no prelude and 13 descriptors", fontFace)
		}
		if _, ok := dump.Atrules["nest"]; ok {
			t.Errorf("Dump().Atrules has nest, want none")
		}

		// upstream dump(false, true)
		prelude, err := lexer.AtrulePrelude("import").Syntax()
		if err != nil {
			t.Fatal(err)
		}
		const wantImport = "[ <string> | <url> ] [ layer | layer( <layer-name> ) ]? [ supports( [ <supports-condition> | <declaration> ] ) ]? <media-query-list>?"
		if got := definitionsyntax.Generate(prelude, definitionsyntax.GenerateOptions{}); got != wantImport {
			t.Errorf("Generate(AtrulePrelude(import)) = %q, want %q", got, wantImport)
		}
	})

	t.Run("String", func(t *testing.T) {
		var dump struct {
			Generic bool              `json:"generic"`
			Types   map[string]string `json:"types"`
			Atrules map[string]struct {
				Prelude     *string           `json:"prelude"`
				Descriptors map[string]string `json:"descriptors"`
			} `json:"atrules"`
		}
		if err := json.Unmarshal([]byte(lexer.String()), &dump); err != nil {
			t.Fatalf("String() is not JSON: %v", err)
		}
		if !dump.Generic || dump.Types["ratio"] != "<number [0,∞]> [/ <number [0,∞]>]?" {
			t.Errorf("String() generic = %v, types.ratio = %q", dump.Generic, dump.Types["ratio"])
		}
		if prelude := dump.Atrules["font-face"].Prelude; prelude != nil {
			t.Errorf("String() atrules.font-face.prelude = %q, want null", *prelude)
		}
	})
}

// TestCheckPlainObjectStructureUnknownType checks that a node type that is
// not a string is written as JavaScript's string conversion writes it, and
// a missing one as undefined.
func TestCheckPlainObjectStructureUnknownType(t *testing.T) {
	lexer := DefaultLexer()

	for _, test := range []struct {
		node map[string]any
		want string
	}{
		{map[string]any{}, "undefined"},
		{map[string]any{"type": nil}, "null"},
		{map[string]any{"type": "x"}, "x"},
		{map[string]any{"type": true}, "true"},
		{map[string]any{"type": 1.0}, "1"},
		{map[string]any{"type": 1.5}, "1.5"},
		{map[string]any{"type": -1.5}, "-1.5"},
		{map[string]any{"type": math.Copysign(0, -1)}, "0"},
		{map[string]any{"type": 0.1}, "0.1"},
		{map[string]any{"type": 100.0}, "100"},
		{map[string]any{"type": 1e6}, "1000000"},
		{map[string]any{"type": 12345.678}, "12345.678"},
		{map[string]any{"type": 1e20}, "100000000000000000000"},
		{map[string]any{"type": 123456789012345680000.0}, "123456789012345680000"},
		{map[string]any{"type": 1e21}, "1e+21"},
		{map[string]any{"type": -1e21}, "-1e+21"},
		{map[string]any{"type": 0.000001}, "0.000001"},
		{map[string]any{"type": 1e-7}, "1e-7"},
		{map[string]any{"type": 1.2345678901234568e-7}, "1.2345678901234568e-7"},
		{map[string]any{"type": 5e-324}, "5e-324"},
		{map[string]any{"type": math.MaxFloat64}, "1.7976931348623157e+308"},
		{map[string]any{"type": map[string]any{}}, "[object Object]"},
		{map[string]any{"type": []any{}}, ""},
		{map[string]any{"type": []any{nil}}, ""},
		{map[string]any{"type": []any{1.0, 2.0}}, "1,2"},
		{map[string]any{"type": []any{[]any{1.0, []any{2.0}}, nil, "a", map[string]any{}, true}}, "1,2,,a,[object Object],true"},
	} {
		warnings := lexer.CheckPlainObjectStructure(test.node)
		want := "Unknown node type `" + test.want + "`"
		if len(warnings) != 1 || warnings[0].Message != want {
			t.Errorf("CheckPlainObjectStructure(%v) = %v, want [%s]", test.node, warnings, want)
		}
	}

	// the structure is looked up by the type converted to a string
	for _, test := range []struct {
		node map[string]any
		want []string
	}{
		{
			map[string]any{"type": []any{"Dimension"}, "loc": nil, "value": "1", "unit": "px"},
			[]string{"Wrong node type `Dimension`, expected `Dimension`"},
		},
		{
			map[string]any{"type": []any{[]any{"Dimension"}}, "value": "1", "unit": "px"},
			[]string{"Wrong node type `Dimension`, expected `Dimension`", "Field `Dimension.loc` is missed"},
		},
		{
			map[string]any{"type": "Dimension", "loc": "x", "value": "1", "unit": "px"},
			[]string{"Bad value for `Dimension.loc`"},
		},
	} {
		var got []string
		for _, warning := range lexer.CheckPlainObjectStructure(test.node) {
			got = append(got, warning.Message)
		}
		if !slices.Equal(got, test.want) {
			t.Errorf("CheckPlainObjectStructure(%v) = %q, want %q", test.node, got, test.want)
		}
	}
}

// TestSyntaxMatchErrorLocation checks the location of a mismatch after
// line breaks (\n, \r\n and \f each end a line).
func TestSyntaxMatchErrorLocation(t *testing.T) {
	lexer := DefaultLexer()

	for _, test := range []struct {
		value      string
		ast        bool
		start, end tokenizerPosition
	}{
		{"red\nblue", false, tokenizerPosition{4, 2, 1}, tokenizerPosition{8, 2, 5}},
		{"red\r\n\fblue", false, tokenizerPosition{6, 3, 1}, tokenizerPosition{10, 3, 5}},
		{"red \n  blue", false, tokenizerPosition{7, 2, 3}, tokenizerPosition{11, 2, 7}},
		{"red\n  blue", true, tokenizerPosition{6, 2, 3}, tokenizerPosition{10, 2, 7}},
	} {
		var result matchOutcome
		if test.ast {
			result = outcome(lexer.MatchProperty("color", mustParse(t, test.value, ParseOptions{Context: "value", Positions: true})))
		} else {
			result = outcome(lexer.MatchPropertyString("color", test.value))
		}

		var matchErr *SyntaxMatchError
		if !errors.As(result.err, &matchErr) {
			t.Fatalf("MatchProperty(color, %q) error = %v, want a SyntaxMatchError", test.value, result.err)
		}
		start := tokenizerPosition{matchErr.Loc.Start.Offset, matchErr.Loc.Start.Line, matchErr.Loc.Start.Column}
		end := tokenizerPosition{matchErr.Loc.End.Offset, matchErr.Loc.End.Line, matchErr.Loc.End.Column}
		fields := tokenizerPosition{matchErr.Offset, matchErr.Line, matchErr.Column}
		if start != test.start || end != test.end || fields != test.start || matchErr.Loc.Filename != "<unknown>" {
			t.Errorf("MatchProperty(color, %q) mismatch at %v-%v (fields %v, source %q), want %v-%v", test.value, start, end, fields, matchErr.Loc.Filename, test.start, test.end)
		}
	}
}

// tokenizerPosition is an offset, line and column.
type tokenizerPosition struct {
	offset, line, column int
}

// TestLexerGenericEdges matches generic types at their edges: a range of a
// number whose text is not a number (Number("+") is NaN, which is out of
// any range) and <attr-unit> without a token.
func TestLexerGenericEdges(t *testing.T) {
	runMatchCases(t, DefaultLexer(), []matchCase{
		{kind: "syntax", name: "<number [0,∞]>", value: "+"},
		{kind: "syntax", name: "<number [0,∞]>", value: "e5"},
		{kind: "syntax", name: "<number [-∞,∞]>", value: "-"},
		{kind: "syntax", name: "<attr-unit>", value: ""},
	})
}

// TestLexerFragmentEdges finds fragments of syntaxes without a name, of
// keywords, and of a syntax ending outside the list it starts in.
func TestLexerFragmentEdges(t *testing.T) {
	nodeNames := func(fragments []Fragment) []string {
		var names []string
		for _, fragment := range fragments {
			for node := range fragment.Nodes.All() {
				name := node.Type()
				switch node := node.(type) {
				case *Operator:
					name += ":" + node.Value
				case *Identifier:
					name += ":" + node.Name
				}
				names = append(names, name)
			}
		}
		return names
	}

	lexer := DefaultLexer()
	for _, test := range []struct {
		property, value string
		target          SyntaxRef
		want            []string
	}{
		// a name "" is upstream's omitted name, which matches the syntaxes
		// without one
		{"font", "12px/1.5 serif", SyntaxRef{Kind: "Token"}, []string{"Operator:/"}},
		{"border", "1px solid red", SyntaxRef{Kind: "Keyword", Name: "solid"}, []string{"Identifier:solid"}},
	} {
		value := mustParse(t, test.value, ParseOptions{Context: "value"})
		fragments, err := lexer.FindValueFragments(test.property, value, test.target)
		if err != nil {
			t.Fatal(err)
		}
		if got := nodeNames(fragments); !slices.Equal(got, test.want) {
			t.Errorf("FindValueFragments(%s, %q, %+v) = %v, want %v", test.property, test.value, test.target, got, test.want)
		}
	}

	// <a> starts at 1px in the parentheses and ends at their ")"
	custom := mustFork(t, ExtendFunc(func(prev *Config) *Config {
		prev.Types = map[string]string{"a": "<length> ')'"}
		prev.Properties = map[string]string{"foo": "'(' <a>"}
		return prev
	}))
	value := mustParse(t, "(1px)", ParseOptions{Context: "value"})
	if _, err := custom.Lexer().MatchProperty("foo", value); err != nil {
		t.Fatalf("MatchProperty(foo, (1px)) error = %v", err)
	}
	fragments, err := custom.Lexer().FindValueFragments("foo", value, SyntaxRef{Kind: KindType, Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := nodeNames(fragments), []string{"Dimension"}; !slices.Equal(got, want) {
		t.Errorf("FindValueFragments(foo, (1px), <a>) = %v, want %v", got, want)
	}
}

// TestLexerBrokenReference checks that matching a syntax that refers to a
// type whose syntax does not parse fails (upstream throws the parse
// error).
func TestLexerBrokenReference(t *testing.T) {
	custom := mustFork(t, ExtendFunc(func(prev *Config) *Config {
		prev.Types = map[string]string{"a": "<b>", "b": "[ x"}
		return prev
	}))

	for _, name := range []string{"a", "b"} {
		_, err := custom.Lexer().MatchTypeString(name, "x")
		if err == nil || !strings.HasPrefix(err.Error(), "Expect `]`") {
			t.Errorf("MatchTypeString(%s, x) error = %v, want Expect `]`", name, err)
		}
	}
}

// TestLexerMatchNodeInputs matches ASTs that are not parsed values: no
// node (upstream null, which throws reading its type, except where there
// is nothing to match), single nodes, and declarations.
func TestLexerMatchNodeInputs(t *testing.T) {
	lexer := DefaultLexer()

	for _, test := range []struct {
		desc       string
		result     matchOutcome
		matched    bool
		iterations int
		err        string // "*" is any error
	}{
		{"property nil", outcome(lexer.MatchProperty("color", nil)), false, 0, "*"},
		{"type nil", outcome(lexer.MatchType("length", nil)), false, 0, "*"},
		{"descriptor nil", outcome(lexer.MatchAtruleDescriptor("font-face", "src", nil)), false, 0, "*"},
		{"prelude nil", outcome(lexer.MatchAtrulePrelude("media", nil)), false, 0, "At-rule `@media` should contain a prelude"},
		{"prelude nil layer", outcome(lexer.MatchAtrulePrelude("layer", nil)), true, 12, ""},
		{"prelude nil font-face", outcome(lexer.MatchAtrulePrelude("font-face", nil)), false, 0, ""},
		{"declaration", outcome(lexer.MatchDeclaration(mustParse(t, "color: red", ParseOptions{Context: "declaration"}))), true, 22, ""},
		{"declaration important", outcome(lexer.MatchDeclaration(mustParse(t, "color: red !important", ParseOptions{Context: "declaration"}))), true, 22, ""},
		{"declaration custom", outcome(lexer.MatchDeclaration(mustParse(t, "--x: red", ParseOptions{Context: "declaration"}))), false, 0, ErrCustomProperty.Error()},
		{"not a declaration", outcome(lexer.MatchDeclaration(mustParse(t, "red", ParseOptions{Context: "value"}))), false, 0, ErrNotDeclaration.Error()},
		{"raw", outcome(lexer.MatchProperty("color", &Raw{Value: " red "})), true, 22, ""},
		{"identifier", outcome(lexer.MatchProperty("color", &Identifier{Name: "red"})), true, 22, ""},
		{"number", outcome(lexer.MatchType("number", &Number{Value: "1"})), true, 3, ""},
	} {
		result := test.result
		if got := result.Matched != nil; got != test.matched || result.Iterations != test.iterations {
			t.Errorf("%s: matched = %v after %d iterations, want %v after %d (error %v)", test.desc, got, result.Iterations, test.matched, test.iterations, result.err)
		}
		switch {
		case test.err == "*" && result.err == nil:
			t.Errorf("%s: error = nil, want an error", test.desc)
		case test.err == "" && result.err != nil:
			t.Errorf("%s: error = %v, want nil", test.desc, result.err)
		case test.err != "" && test.err != "*" && (result.err == nil || result.err.Error() != test.err):
			t.Errorf("%s: error = %v, want %q", test.desc, result.err, test.err)
		}
	}
}

// TestLexerMatchDescriptorEdges checks matching against a parsed definition
// syntax, an empty one and missing syntaxes.
func TestLexerMatchDescriptorEdges(t *testing.T) {
	lexer := DefaultLexer()

	tree, err := definitionsyntax.Parse("<length> | auto")
	if err != nil {
		t.Fatal(err)
	}
	descriptor := NewDescriptorFromTree(SyntaxRef{Kind: KindType, Name: "anonymous"}, tree)
	for _, test := range []struct {
		value      string
		matched    bool
		iterations int
	}{
		{"auto", true, 8},
		{"1px", true, 6},
		{"none", false, 8},
	} {
		result := outcome(lexer.MatchDescriptorString(descriptor, test.value))
		if got := result.Matched != nil; got != test.matched || result.Iterations != test.iterations {
			t.Errorf("MatchDescriptorString(<length> | auto, %q) = %v, %d iterations, want %v, %d", test.value, got, result.Iterations, test.matched, test.iterations)
		}
		var matchErr *SyntaxMatchError
		if errors.As(result.err, &matchErr) && matchErr.Syntax != "<length> | auto" {
			t.Errorf("MatchDescriptorString(<length> | auto, %q) syntax = %q", test.value, matchErr.Syntax)
		}
	}

	result, err := lexer.MatchString("", "x")
	var matchErr *SyntaxMatchError
	if !errors.As(err, &matchErr) || matchErr.RawMessage != "Mismatch" || matchErr.Syntax != "" || result.Iterations != 3 {
		t.Errorf(`MatchString("", "x") = %v, %d iterations, want a Mismatch of "" after 3`, err, result.Iterations)
	}

	for _, test := range []struct {
		desc   string
		result matchOutcome
		want   string
	}{
		{"unknown type reference", outcome(lexer.MatchString("<unknown-type>", "x")), "Bad syntax reference: <unknown-type>"},
		{"nil descriptor", outcome(lexer.MatchDescriptorString(nil, "x")), "Bad syntax"},
		{"unknown type", outcome(lexer.MatchTypeString("nope", "x")), "Unknown type `nope`"},
		{"unknown property", outcome(lexer.MatchPropertyString("nope", "x")), "Unknown property `nope`"},
		{"media descriptor", outcome(lexer.MatchAtruleDescriptorString("media", "x", "x")), "At-rule `@media` has no known descriptors"},
		{"unknown descriptor", outcome(lexer.MatchAtruleDescriptorString("font-face", "nope", "x")), "Unknown at-rule descriptor `nope`"},
		{"unknown at-rule", outcome(lexer.MatchAtrulePreludeString("nope", "x")), "Unknown at-rule `@nope`"},
	} {
		if test.result.Matched != nil || test.result.err == nil || test.result.err.Error() != test.want {
			t.Errorf("%s: error = %v, want %q", test.desc, test.result.err, test.want)
		}
	}

	var refErr *SyntaxReferenceError
	if _, err := lexer.MatchTypeString("nope", "x"); !errors.As(err, &refErr) || refErr.Reference != "nope" {
		t.Errorf(`MatchTypeString("nope", "x") error = %v, want a SyntaxReferenceError for nope`, err)
	}

	if result, err := lexer.MatchAtrulePreludeString("font-face", ""); result.Matched != nil || err != nil {
		t.Errorf(`MatchAtrulePreludeString("font-face", "") = %v, %v, want no match and no error`, result.Matched, err)
	}

	pair := NewDescriptor(SyntaxRef{Kind: KindType, Name: "pair"}, "<number> <number>")
	if pair.Kind() != KindType || pair.Name() != "pair" || pair.Parent() != "" || !pair.Serializable() {
		t.Errorf("NewDescriptor(pair) = %s %q parent %q serializable %v, want Type pair", pair.Kind(), pair.Name(), pair.Parent(), pair.Serializable())
	}
	for _, value := range []string{"1 2", "3 4"} {
		if _, err := lexer.MatchDescriptorString(pair, value); err != nil {
			t.Errorf("MatchDescriptorString(pair, %q) error = %v", value, err)
		}
	}
	if _, err := lexer.MatchDescriptorString(pair, "1"); err == nil {
		t.Errorf(`MatchDescriptorString(pair, "1") = a match, want an error`)
	}
	bad := NewDescriptor(SyntaxRef{Kind: KindType, Name: "bad"}, "<number")
	if _, err := lexer.MatchDescriptorString(bad, "1"); err == nil {
		t.Errorf(`MatchDescriptorString(<number, "1") = a match, want a parse error`)
	}

	// a group with a combinator the definition syntax parser never
	// produces fails to build (upstream builds an undefined graph)
	unknownCombinator := NewDescriptorFromTree(SyntaxRef{Kind: KindType, Name: "anonymous"}, &definitionsyntax.Group{
		Combinator: "x",
		Terms:      []definitionsyntax.Node{&definitionsyntax.Keyword{Name: "a"}},
	})
	if result, err := lexer.MatchDescriptorString(unknownCombinator, "a"); result.Matched != nil || err == nil {
		t.Errorf(`MatchDescriptorString(group with combinator "x", "a") = %v, %v, want an error`, result.Matched, err)
	}
}

// TestLexerFunctionFragmentsAndTrace finds the fragments and traces of
// function syntaxes.
func TestLexerFunctionFragmentsAndTrace(t *testing.T) {
	lexer := DefaultLexer()
	value := mustParse(t, "translate(1px) rotate(1deg)", ParseOptions{Context: "value"})

	fragmentNames := func(fragments []Fragment) []string {
		var names []string
		for _, fragment := range fragments {
			for node := range fragment.Nodes.All() {
				names = append(names, node.Type()+":"+node.(*Function).Name)
			}
		}
		return names
	}

	for _, test := range []struct {
		target SyntaxRef
		want   []string
	}{
		{SyntaxRef{Kind: "Function", Name: "translate"}, []string{"Function:translate"}},
		{SyntaxRef{Kind: KindType, Name: "transform-function"}, []string{"Function:translate", "Function:rotate"}},
		{SyntaxRef{Kind: "Keyword", Name: "translate"}, nil},
	} {
		fragments, err := lexer.FindValueFragments("transform", value, test.target)
		if err != nil {
			t.Fatal(err)
		}
		if got := fragmentNames(fragments); !slices.Equal(got, test.want) {
			t.Errorf("FindValueFragments(transform, %+v) = %v, want %v", test.target, got, test.want)
		}
	}

	match := outcome(lexer.MatchProperty("transform", value))
	translate := value.(*Value).Children.First()
	length := translate.(*Function).Children.First()

	traceNames := func(trace []MatchSyntax) []string {
		names := []string{}
		for _, syntax := range trace {
			names = append(names, syntax.Type()+":"+SyntaxName(syntax))
		}
		return names
	}

	wantTranslate := []string{"Property:transform", "Type:transform-list", "Type:transform-function", "Type:translate()"}
	if got := traceNames(match.Trace(translate)); !slices.Equal(got, wantTranslate) {
		t.Errorf("Trace(translate()) = %v, want %v", got, wantTranslate)
	}
	wantLength := append(slices.Clone(wantTranslate), "Type:length-percentage", "Type:length")
	if got := traceNames(match.Trace(length)); !slices.Equal(got, wantLength) {
		t.Errorf("Trace(1px) = %v, want %v", got, wantLength)
	}
	if got := match.Trace(mustParse(t, "1px", ParseOptions{Context: "value"})); got != nil {
		t.Errorf("Trace(another node) = %v, want nil", got)
	}

	for _, test := range []struct {
		desc string
		got  bool
		want bool
	}{
		{"IsType(translate(), transform-function)", match.IsType(translate, "transform-function"), true},
		{"IsKeyword(translate())", match.IsKeyword(translate), false},
		{"IsProperty(translate(), transform)", match.IsProperty(translate, "transform"), true},
		{"IsProperty(translate(), width)", match.IsProperty(translate, "width"), false},
		{"IsType(1px, length)", match.IsType(length, "length"), true},
		{"IsType(1px, length-percentage)", match.IsType(length, "length-percentage"), true},
		{"IsType(1px, angle)", match.IsType(length, "angle"), false},
	} {
		if test.got != test.want {
			t.Errorf("%s = %v, want %v", test.desc, test.got, test.want)
		}
	}
}
