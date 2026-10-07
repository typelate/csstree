// Go only.

package csstree

import (
	"errors"
	"runtime"
	"testing"

	"github.com/typelate/csstree/tokenizer"
)

// Upstream's parseWithFallback catches any exception, not only syntax
// errors. In Go a panic that is not a *SyntaxError is a bug (here a nil
// dereference in an extension): falling back to a Raw node would hide it,
// so Parse returns it as an *InternalError instead, and never crashes the
// caller.
func TestParseReportsPanics(t *testing.T) {
	extended, err := Fork(ExtendFunc(func(syntaxConfig *Config) *Config {
		defaultGetNode := syntaxConfig.Scope["Value"].GetNode

		syntaxConfig.Scope["Value"].GetNode = func(p *Parser, context *SequenceContext) Node {
			switch {
			case p.IsDelim(dollarSign, 0):
				var raw *Raw
				return &Raw{Value: raw.Value} // nil dereference
			case p.IsDelim('%', 0):
				p.Fail("Percent is not allowed")
			}

			return defaultGetNode(p, context)
		}

		return syntaxConfig
	}))
	if err != nil {
		t.Fatalf("Fork(...): %v", err)
	}

	assertInternalError := func(t *testing.T, err error) {
		t.Helper()
		var internal *InternalError
		if !errors.As(err, &internal) {
			t.Fatalf("error = %v, want an *InternalError", err)
		}
		if len(internal.Stack) == 0 {
			t.Error("InternalError.Stack is empty")
		}
	}

	t.Run("a panic inside parseWithFallback is returned", func(t *testing.T) {
		var reported []error
		ast, err := extended.Parse("color: $a", ParseOptions{
			Context: "declaration",
			OnParseError: func(err error, _ Node) error {
				reported = append(reported, err)
				return nil
			},
		})
		if ast != nil {
			t.Errorf("Parse(%q) = %#v, want nil", "color: $a", ast)
		}
		assertInternalError(t, err)
		var runtimeError runtime.Error
		if !errors.As(err, &runtimeError) {
			t.Errorf("Parse(%q) error = %v, want a runtime.Error", "color: $a", err)
		}
		if len(reported) != 0 {
			t.Errorf("OnParseError reported %v, want nothing", reported)
		}
	})

	t.Run("a panic outside parseWithFallback is returned", func(t *testing.T) {
		_, err := extended.Parse("$a", ParseOptions{Context: "value"})
		assertInternalError(t, err)
	})

	t.Run("a syntax error of an extension falls back to Raw", func(t *testing.T) {
		var reported error
		ast, err := extended.Parse("color: %a", ParseOptions{
			Context: "declaration",
			OnParseError: func(err error, _ Node) error {
				reported = err
				return nil
			},
		})
		if err != nil {
			t.Fatalf("Parse(%q) error = %v, want nil", "color: %a", err)
		}
		if raw, ok := ast.(*Declaration).Value.(*Raw); !ok || raw.Value != "%a" {
			t.Errorf("Parse(%q).Value = %#v, want Raw %%a", "color: %a", ast.(*Declaration).Value)
		}
		var syntaxErr *SyntaxError
		if !errors.As(reported, &syntaxErr) || syntaxErr.Message != "Percent is not allowed" {
			t.Errorf("OnParseError error = %v, want the extension's *SyntaxError", reported)
		}
	})

	t.Run("a panic in OnToken is returned", func(t *testing.T) {
		_, err := Parse("a{}", ParseOptions{OnToken: func(*TokenIterateAPI, tokenizer.TokenType, int, int, int) {
			panic("boom")
		}})
		assertInternalError(t, err)
	})

	t.Run("a panic in OnComment is returned", func(t *testing.T) {
		_, err := Parse("/* c */", ParseOptions{OnComment: func(string, *Location) {
			panic("boom")
		}})
		assertInternalError(t, err)
	})
}
