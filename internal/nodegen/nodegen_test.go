package nodegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func generateSource(t *testing.T, source string) ([]byte, error) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "node_test_input.go", "package csstree\n"+source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return generate(fset, []*ast.File{file})
}

func TestGenerate(t *testing.T) {
	source, err := generateSource(t, `
// A is a node.
//
//csstree:node name=Alpha walkContext=alpha
type A struct {
	Loc       *Location
	Name      string      `+"`css:\"name,string|null\"`"+`
	Important string      `+"`css:\"important,boolean|string,json=toJSON,read=readImportant\"`"+`
	Prelude   Node        `+"`css:\"prelude,B|Raw|null\"`"+`
	Block     *B          `+"`css:\"block,Bravo\"`"+`
	Children  *List[Node] `+"`css:\"children,[Bravo|Raw]|null\"`"+`
}

//csstree:node name=Bravo
type B struct {
	Loc *Location
}

// C is not a node.
type C struct {
	Loc *Location
}
`)
	if err != nil {
		t.Fatalf("generate() error = %v", err)
	}

	for _, want := range []string{
		`func (n *A) Type() string { return "Alpha" }`,
		`jsonField{"name", nullableString(n.Name)},`,
		`jsonField{"important", toJSON(n.Important)},`,
		`{Name: "important", Types: []FieldType{booleanField(), stringField()}},`,
		`{Name: "prelude", Types: []FieldType{nodeField("B"), nodeField("Raw"), nullField()}},`,
		`{Name: "children", Types: []FieldType{listField("Bravo", "Raw"), nullField()}},`,
		`WalkContext: "alpha",`,
		"if reverse {\n\t\treturn n.Children != nil && walkList(n.Children) ||\n\t\t\tn.Block != nil && walkNode(n.Block) ||\n\t\t\tn.Prelude != nil && walkNode(n.Prelude)\n\t}",
		"if c.Block != nil {\n\t\tc.Block = cloneAs(c.Block, clone)\n\t}",
		`Important: readImportant(r, object, "important"),`,
		`Block:     PlainObjectNode[*B](r, object, "block"),`,
		`func (n *B) MarshalJSON() ([]byte, error) {` + "\n\treturn marshalNode(n)\n}",
		`Structure:       []StructureField{},`,
		`func cloneB(node Node, _ func(Node) Node) Node {`,
	} {
		if !strings.Contains(string(source), want) {
			t.Errorf("generate() does not contain\n%s\nin\n%s", want, source)
		}
	}
	for _, unwanted := range []string{"func walkB(", "(n *C)"} {
		if strings.Contains(string(source), unwanted) {
			t.Errorf("generate() contains %q", unwanted)
		}
	}
}

func TestGenerateErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "no Loc",
			source: "//csstree:node\ntype A struct{ Name string `css:\"name,string\"` }",
			want:   "the first field must be Loc *Location",
		},
		{
			name:   "no tag",
			source: "//csstree:node\ntype A struct{ Loc *Location; Name string }",
			want:   "field Name has no css tag",
		},
		{
			name:   "another tag key",
			source: "//csstree:node\ntype A struct{ Loc *Location; Name string `json:\"name\"` }",
			want:   "is not a css tag",
		},
		{
			name:   "no field types",
			source: "//csstree:node\ntype A struct{ Loc *Location; Name string `css:\"name\"` }",
			want:   "does not start with a name and the field types",
		},
		{
			name:   "unknown option",
			source: "//csstree:node\ntype A struct{ Loc *Location; Name string `css:\"name,string,omitempty\"` }",
			want:   `unknown option "omitempty"`,
		},
		{
			name:   "unterminated list",
			source: "//csstree:node\ntype A struct{ Loc *Location; Children *List[Node] `css:\"children,[A|B\"` }",
			want:   "unterminated list",
		},
		{
			name:   "lower case node type",
			source: "//csstree:node\ntype A struct{ Loc *Location; Value Node `css:\"value,raw\"` }",
			want:   `"raw" is not a field type`,
		},
		{
			name:   "node type on a string",
			source: "//csstree:node\ntype A struct{ Loc *Location; Name string `css:\"name,Identifier\"` }",
			want:   "field type Identifier does not fit Go type string",
		},
		{
			name:   "string on a Node",
			source: "//csstree:node\ntype A struct{ Loc *Location; Value Node `css:\"value,string\"` }",
			want:   "field type string does not fit Go type Node",
		},
		{
			name:   "null only",
			source: "//csstree:node\ntype A struct{ Loc *Location; Value Node `css:\"value,null\"` }",
			want:   "do not fit Go type Node",
		},
		{
			name:   "wrong node type for a pointer",
			source: "//csstree:node\ntype A struct{ Loc *Location; Next *A `css:\"next,B\"` }",
			want:   "field type B does not fit Go type *A",
		},
		{
			name:   "pointer to a struct that is not a node",
			source: "//csstree:node\ntype A struct{ Loc *Location; Next *C `css:\"next,C\"` }\ntype C struct{ Loc *Location }",
			want:   `type "*C" is not string, Node, *List[Node] or a pointer to a node type`,
		},
		{
			name:   "two lists",
			source: "//csstree:node\ntype A struct{ Loc *Location; Children *List[Node] `css:\"children,[]|[A]\"` }",
			want:   "do not fit Go type *List[Node]",
		},
		{
			name:   "json without read",
			source: "//csstree:node\ntype A struct{ Loc *Location; Name string `css:\"name,boolean|string,json=f\"` }",
			want:   "json and read go together on a string field",
		},
		{
			name:   "unknown directive argument",
			source: "//csstree:node context=a\ntype A struct{ Loc *Location }",
			want:   `unknown directive argument "context=a"`,
		},
		{
			name:   "duplicate node type",
			source: "//csstree:node\ntype A struct{ Loc *Location }\n//csstree:node name=A\ntype B struct{ Loc *Location }",
			want:   "node type A is declared by A and B",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := generateSource(t, tt.source)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("generate(%q) error = %v, want one containing %q", tt.source, err, tt.want)
			}
		})
	}
}
