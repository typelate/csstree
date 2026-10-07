package csstree

import (
	"errors"
	"testing"

	"github.com/typelate/csstree/tokenizer"
)

// Expected values in this file are upstream's (csstree 3.2.1 with
// source-map-js).

func TestSourceMapSourcesFileAndContent(t *testing.T) {
	a := mustParse(t, "a{b:c}\n@media x{d{e:f}}", ParseOptions{Positions: true, Filename: "a.css"}).(*StyleSheet)
	b := mustParse(t, "g{h:i}", ParseOptions{Positions: true, Filename: "b\"\\\n\t\u0001\u001f\u007fé.css"}).(*StyleSheet)
	c := mustParse(t, "j{k:l}", ParseOptions{Positions: true, Filename: "a.css"}).(*StyleSheet)
	a.Children.AppendList(b.Children)
	a.Children.AppendList(c.Children)

	css, sourceMap, err := GenerateWithSourceMap(a, GenerateOptions{})
	if err != nil {
		t.Fatalf("GenerateWithSourceMap(a, ...): %v", err)
	}
	sourceMap.File = "out\n.css"
	sourceMap.SetSourceContent("a.css", "a{b:c}\n\"x\"")

	if want := "a{b:c}@media x{d{e:f}}g{h:i}j{k:l}"; css != want {
		t.Errorf("css = %q, want %q", css, want)
	}
	want := `{"version":3,"sources":["a.css","b\"\\\n\t\u0001\u001f` + "\u007f" + `é.css"],"names":[],"mappings":"AAAA,C,CAAE,G,CACF,SAAS,C,CAAE,KCDX,C,CAAE,G,CDAF,C,CAAE,G","file":"out\n.css","sourcesContent":["a{b:c}\n\"x\"",null]}`
	if got := sourceMap.String(); got != want {
		t.Errorf("map =\n%s\nwant\n%s", got, want)
	}
	data, err := sourceMap.MarshalJSON()
	if err != nil {
		t.Fatalf("sourceMap.MarshalJSON(): %v", err)
	}
	if string(data) != want {
		t.Errorf("MarshalJSON() = %s, want %s", data, want)
	}
}

// A location that is not a source position fails the source map, as
// upstream's addMapping throws.
func TestGenerateWithSourceMapBadLocation(t *testing.T) {
	loc := &Location{Start: tokenizer.Position{Line: 1, Column: 1}}
	css, sourceMap, err := GenerateWithSourceMap(&Declaration{Property: "a", Value: &Raw{Value: "b"}, Loc: loc}, GenerateOptions{})
	want := `csstree: invalid source mapping: {"generated":{"line":1,"column":0},"source":null,"original":{"line":1,"column":0},"name":null}`
	if css != "" || sourceMap != nil || err == nil || err.Error() != want {
		t.Errorf("GenerateWithSourceMap() = %q, %v, %v; want %s", css, sourceMap, err, want)
	}
}

func TestSourceMapGeneratorAddMapping(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		for _, tt := range []struct {
			mapping sourceMapping
			want    string
		}{
			{
				mapping: sourceMapping{Generated: sourceMapPosition{Line: 0, Column: 0}},
				want:    `csstree: invalid source mapping: {"generated":{"line":0,"column":0},"source":null,"original":null,"name":null}`,
			},
			{
				mapping: sourceMapping{Generated: sourceMapPosition{Line: 1, Column: -1}},
				want:    `csstree: invalid source mapping: {"generated":{"line":1,"column":-1},"source":null,"original":null,"name":null}`,
			},
			{
				mapping: sourceMapping{Generated: sourceMapPosition{Line: 1}, Source: "x"},
				want:    `csstree: invalid source mapping: {"generated":{"line":1,"column":0},"source":"x","original":null,"name":null}`,
			},
			{
				mapping: sourceMapping{Generated: sourceMapPosition{Line: 1}, Original: &sourceMapPosition{Line: 1}},
				want:    `csstree: invalid source mapping: {"generated":{"line":1,"column":0},"source":null,"original":{"line":1,"column":0},"name":null}`,
			},
			{
				mapping: sourceMapping{Generated: sourceMapPosition{Line: 1}, Original: &sourceMapPosition{Line: 0}, Source: "x"},
				want:    `csstree: invalid source mapping: {"generated":{"line":1,"column":0},"source":"x","original":{"line":0,"column":0},"name":null}`,
			},
			{
				mapping: sourceMapping{Generated: sourceMapPosition{Line: 1}, Original: &sourceMapPosition{Line: 1, Column: -1}, Source: "x", Name: `n"`},
				want:    `csstree: invalid source mapping: {"generated":{"line":1,"column":0},"source":"x","original":{"line":1,"column":-1},"name":"n\""}`,
			},
			{
				mapping: sourceMapping{Generated: sourceMapPosition{Line: 1}, Name: "n"},
				want:    `csstree: invalid source mapping: {"generated":{"line":1,"column":0},"source":null,"original":null,"name":"n"}`,
			},
		} {
			var g SourceMapGenerator
			err := g.addMapping(tt.mapping)
			if err == nil || err.Error() != tt.want {
				t.Errorf("addMapping(%+v) = %v, want %s", tt.mapping, err, tt.want)
			}
		}
	})

	t.Run("names, order and duplicates", func(t *testing.T) {
		var g SourceMapGenerator
		for _, m := range []sourceMapping{
			{Generated: sourceMapPosition{Line: 2, Column: 5}, Original: &sourceMapPosition{Line: 1, Column: 0}, Source: "z", Name: "n1"},
			{Generated: sourceMapPosition{Line: 1, Column: 3}, Original: &sourceMapPosition{Line: 3, Column: 7}, Source: "y", Name: "n2"},
			{Generated: sourceMapPosition{Line: 1, Column: 3}, Original: &sourceMapPosition{Line: 3, Column: 7}, Source: "y", Name: "n2"},
			{Generated: sourceMapPosition{Line: 1, Column: 3}, Original: &sourceMapPosition{Line: 3, Column: 7}, Source: "x"},
			{Generated: sourceMapPosition{Line: 1, Column: 3}},
			{Generated: sourceMapPosition{Line: 1, Column: 3}, Original: &sourceMapPosition{Line: 2, Column: 7}, Source: "y", Name: "n1"},
			{Generated: sourceMapPosition{Line: 1, Column: 3}, Original: &sourceMapPosition{Line: 3, Column: 6}, Source: "y"},
			{Generated: sourceMapPosition{Line: 1, Column: 1000}, Original: &sourceMapPosition{Line: 300, Column: 0}, Source: "y"},
		} {
			if err := g.addMapping(m); err != nil {
				t.Fatalf("g.AddMapping(m): %v", err)
			}
		}
		g.SetSourceContent("y", "yy")

		want := `{"version":3,"sources":["z","y","x"],"names":["n1","n2"],"mappings":"GEEO,ADDAA,AACD,AAACC,A,q+BAySP;KD3SAD","sourcesContent":[null,"yy",null]}`
		if got := g.String(); got != want {
			t.Errorf("map =\n%s\nwant\n%s", got, want)
		}
	})
}

func TestGenerateTokens(t *testing.T) {
	value := func(children ...Node) Node {
		list := new(List[Node])
		for _, child := range children {
			list.AppendData(child)
		}
		return &Value{Children: list}
	}
	for _, tt := range []struct {
		name string
		node Node
		mode GenerateMode
		want string
	}{
		// a lone \ is followed by a newline so it cannot escape what comes next
		{name: "reverse solidus", node: value(&Raw{Value: `\`}, &Identifier{Name: "a"}), want: "\\\na"},
		{name: "escape", node: value(&Identifier{Name: "a"}, &Raw{Value: `\b`}), want: `a \b`},
		{name: "ident hash safe", node: value(&Identifier{Name: "a"}, &Hash{Value: "b"}), mode: "safe", want: "a #b"},
		{name: "ident hash spec", node: value(&Identifier{Name: "a"}, &Hash{Value: "b"}), mode: "spec", want: "a#b"},
		{name: "ident hash unknown mode", node: value(&Identifier{Name: "a"}, &Hash{Value: "b"}), mode: "bogus", want: "a #b"},
		{name: "number percentage safe", node: value(&Number{Value: "1"}, &Percentage{Value: "2"}), mode: "safe", want: "1 2%"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			css, err := Generate(tt.node, GenerateOptions{Mode: tt.mode})
			if err != nil {
				t.Fatalf("Generate(tt.node, GenerateOptions{Mode: tt.mode}): %v", err)
			}
			if css != tt.want {
				t.Errorf("Generate() = %q, want %q", css, tt.want)
			}
		})
	}
}

// A panic in a handler ends the generation with an *InternalError, as an
// exception thrown by an upstream handler reaches the caller of generate.
func TestGenerateHandlerPanics(t *testing.T) {
	css, err := defaultParseSyntax().generate(&Identifier{Name: "a"}, "", func(h *generateHandlers) {
		h.emit = func(tokenizer.TokenType, string, bool) { panic("boom") }
	})
	var internal *InternalError
	if css != "" || !errors.As(err, &internal) || internal.Value != "boom" {
		t.Errorf("Generate() = %q, %v; want an *InternalError of boom", css, err)
	}

	// upstream: reading the type of a null child throws
	_, err = Generate(&Value{Children: new(List[Node]).AppendData(nil)}, GenerateOptions{})
	var unknown *UnknownNodeTypeError
	if !errors.As(err, &unknown) || unknown.Type != "undefined" {
		t.Errorf("Generate(nil child) = %v, want *UnknownNodeTypeError", err)
	}
}

func TestEncodeStringAndURL(t *testing.T) {
	for _, tt := range []struct {
		in, quoted, apostrophe, url string
	}{
		{in: "\x00", quoted: `"` + "�" + `"`, apostrophe: `'` + "�" + `'`, url: "url(�)"},
		{in: `a'b"c\d`, quoted: `"a'b\"c\\d"`, apostrophe: `'a\'b"c\\d'`, url: `url(a\'b\"c\\d)`},
		{in: "\x01a", quoted: `"\1 a"`, apostrophe: `'\1 a'`, url: `url(\1 a)`},
		{in: "\x1f ", quoted: `"\1f  "`, apostrophe: `'\1f  '`, url: `url(\1f\ )`},
		{in: "\x7fF", quoted: `"\7f F"`, apostrophe: `'\7f F'`, url: `url(\7f F)`},
		{in: "é\x00😀", quoted: "\"é�😀\"", apostrophe: "'é�😀'", url: "url(é�😀)"},
	} {
		if got := EncodeString(tt.in, false); got != tt.quoted {
			t.Errorf("EncodeString(%q, false) = %q, want %q", tt.in, got, tt.quoted)
		}
		if got := EncodeString(tt.in, true); got != tt.apostrophe {
			t.Errorf("EncodeString(%q, true) = %q, want %q", tt.in, got, tt.apostrophe)
		}
		if got := EncodeURL(tt.in); got != tt.url {
			t.Errorf("EncodeURL(%q) = %q, want %q", tt.in, got, tt.url)
		}
	}
}

func TestAtruleDefinitionMarshalJSON(t *testing.T) {
	for _, tt := range []struct {
		definition AtruleDefinition
		want       string
	}{
		{definition: AtruleDefinition{}, want: `{"prelude":null,"descriptors":null}`},
		{
			definition: AtruleDefinition{Prelude: "<a> & <b>", Descriptors: map[string]string{"x": "<c>"}},
			want:       `{"prelude":"<a> & <b>","descriptors":{"x":"<c>"}}`,
		},
	} {
		data, err := tt.definition.MarshalJSON()
		if err != nil {
			t.Fatalf("tt.definition.MarshalJSON(): %v", err)
		}
		if string(data) != tt.want {
			t.Errorf("MarshalJSON(%+v) = %s, want %s", tt.definition, data, tt.want)
		}
	}
}

// Clone copies every node, list and location of every AST fixture.
func TestCloneASTFixtures(t *testing.T) {
	forEachASTTest(t, false, func(t *testing.T, test astFixtureTest) {
		options := test.ParseOptions(false)
		options.Positions = true
		ast, err := Parse(test.Source, options)
		if err != nil {
			t.Fatalf("Parse(%q, options): %v", test.Source, err)
		}
		astCopy := Clone(ast)

		if got, want := jsonText(t, toPlainJSON(t, astCopy), "    "), jsonText(t, toPlainJSON(t, ast), "    "); got != want {
			t.Fatalf("Clone() =\n%s\nwant\n%s", got, want)
		}

		originals := map[any]bool{}
		mustWalk(t, ast, enterOnly(func(node Node) {
			originals[node] = true
			if loc := node.Location(); loc != nil {
				originals[loc] = true
			}
		}))
		mustWalk(t, astCopy, enterOnly(func(node Node) {
			if originals[node] {
				t.Errorf("Clone() shares %s node %v", node.Type(), node)
			}
			if loc := node.Location(); loc != nil && originals[loc] {
				t.Errorf("Clone() shares the location of %s node", node.Type())
			}
		}))
	})
}

// Go only: a hand-built node with nil fields generates or fails with an
// *UnknownNodeTypeError (a missing node), never with an *InternalError (a
// nil dereference); nil lists generate as empty lists.
func TestGenerateZeroNodes(t *testing.T) {
	for _, node := range []Node{
		&AnPlusB{},
		&AtrulePrelude{},
		&Atrule{},
		&AttributeSelector{},
		&Block{},
		&Brackets{},
		&CDC{},
		&CDO{},
		&ClassSelector{},
		&Combinator{},
		&Comment{},
		&Condition{},
		&DeclarationList{},
		&Declaration{},
		&Dimension{},
		&FeatureFunction{},
		&FeatureRange{},
		&Feature{},
		&Function{},
		&GeneralEnclosed{},
		&Hash{},
		&IDSelector{},
		&Identifier{},
		&LayerList{},
		&Layer{},
		&MediaQueryList{},
		&MediaQuery{},
		&NestingSelector{},
		&Nth{},
		&Number{},
		&Operator{},
		&Parentheses{},
		&Percentage{},
		&PseudoClassSelector{},
		&PseudoElementSelector{},
		&Ratio{},
		&Raw{},
		&Rule{},
		&Scope{},
		&SelectorList{},
		&Selector{},
		&String{},
		&StyleSheet{},
		&SupportsDeclaration{},
		&TypeSelector{},
		&UnicodeRange{},
		&URL{},
		&Value{},
		&WhiteSpace{},
	} {
		t.Run(node.Type(), func(t *testing.T) {
			css, err := Generate(node, GenerateOptions{})
			var unknown *UnknownNodeTypeError
			if err != nil && !errors.As(err, &unknown) {
				t.Errorf("Generate(&%s{}) = %q, %v; want no error or an *UnknownNodeTypeError", node.Type(), css, err)
			}
		})
	}

	if css, err := Generate(&Rule{Prelude: &SelectorList{}}, GenerateOptions{}); err == nil {
		t.Errorf("Generate(Rule without Block) = %q, want an error", css)
	}
	if css, err := Generate(&Rule{Prelude: &SelectorList{}, Block: &Block{}}, GenerateOptions{}); err != nil || css != "{}" {
		t.Errorf("Generate(Rule with empty lists) = %q, %v; want {}", css, err)
	}
}
