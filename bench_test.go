package csstree

import (
	"os"
	"strings"
	"testing"

	"github.com/typelate/csstree/tokenizer"
)

// benchmarkInput is a stylesheet the benchmarks parse, generate or tokenize.
type benchmarkInput struct {
	name   string
	source string
}

// benchmarkInputs returns testdata/stringify.css and a larger stylesheet
// made of the sources of the stylesheet, rule and atrule AST fixtures that
// end where they end (see endsItself), repeated to at least 256 KiB.
func benchmarkInputs(b *testing.B) []benchmarkInput {
	b.Helper()

	data, err := os.ReadFile(stringifyFixtureFilename)
	if err != nil {
		b.Fatalf("os.ReadFile(%q): %v", stringifyFixtureFilename, err)
	}

	var fixtures strings.Builder
	for _, file := range loadASTFixtures(b, []string{"stylesheet", "rule", "atrule"}) {
		for _, test := range file.Tests {
			if test.Skip || !endsItself(b, test.Source) {
				continue
			}
			fixtures.WriteString(test.Source)
			fixtures.WriteString("\n")
		}
	}
	unit := fixtures.String()
	if unit == "" {
		b.Fatal("no stylesheet, rule or atrule AST fixtures")
	}
	const minSize = 256 << 10
	large := strings.Repeat(unit, (minSize+len(unit)-1)/len(unit))

	return []benchmarkInput{
		{name: "stringify.css", source: string(data)},
		{name: "fixtures", source: large},
	}
}

// endsItself reports whether a rule after source parses as a rule of its
// own, unlike after an unclosed comment, block or parenthesis.
func endsItself(b *testing.B, source string) bool {
	b.Helper()
	const next = ".next{}"
	ast, err := Parse(source+"\n"+next, ParseOptions{})
	if err != nil {
		b.Fatalf("Parse(%q): %v", source, err)
	}
	css, err := Generate(ast, GenerateOptions{})
	if err != nil {
		b.Fatalf("Generate(Parse(%q)): %v", source, err)
	}
	return strings.HasSuffix(css, "}"+next) || strings.HasSuffix(css, ";"+next) || css == next
}

func BenchmarkTokenize(b *testing.B) {
	for _, input := range benchmarkInputs(b) {
		b.Run(input.name, func(b *testing.B) {
			b.SetBytes(int64(len(input.source)))
			b.ReportAllocs()
			for b.Loop() {
				tokenizer.Tokenize(input.source, func(tokenizer.TokenType, int, int) {})
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	for _, input := range benchmarkInputs(b) {
		b.Run(input.name, func(b *testing.B) {
			b.SetBytes(int64(len(input.source)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Parse(input.source, ParseOptions{}); err != nil {
					b.Fatalf("Parse(%s): %v", input.name, err)
				}
			}
		})
	}
}

func BenchmarkGenerate(b *testing.B) {
	for _, input := range benchmarkInputs(b) {
		b.Run(input.name, func(b *testing.B) {
			ast, err := Parse(input.source, ParseOptions{})
			if err != nil {
				b.Fatalf("Parse(%s): %v", input.name, err)
			}
			b.SetBytes(int64(len(input.source)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Generate(ast, GenerateOptions{}); err != nil {
					b.Fatalf("Generate(Parse(%s)): %v", input.name, err)
				}
			}
		})
	}
}

func BenchmarkGenerateSourceMap(b *testing.B) {
	for _, input := range benchmarkInputs(b) {
		b.Run(input.name, func(b *testing.B) {
			ast, err := Parse(input.source, ParseOptions{Filename: input.name, Positions: true})
			if err != nil {
				b.Fatalf("Parse(%s, positions): %v", input.name, err)
			}
			b.SetBytes(int64(len(input.source)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Generate(ast, GenerateOptions{SourceMap: new(SourceMapGenerator)}); err != nil {
					b.Fatalf("Generate(Parse(%s), source map): %v", input.name, err)
				}
			}
		})
	}
}
