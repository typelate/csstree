// Ported from lib/definition-syntax/generate.js.

package definitionsyntax

import (
	"strconv"
	"strings"
)

// GenerateOptions configures Generate. The zero value generates the
// canonical form.
type GenerateOptions struct {
	// ForceBraces wraps every group in square brackets.
	ForceBraces bool
	// Compact omits optional whitespace.
	Compact bool
	// Decorate, when set, is called with the text generated for each node
	// and returns the text to use instead.
	Decorate func(str string, node Node) string
}

// decorate returns Decorate(str, node), or str when Decorate is nil.
func (o GenerateOptions) decorate(str string, node Node) string {
	if o.Decorate == nil {
		return str
	}
	return o.Decorate(str, node)
}

func generateMultiplier(multiplier *Multiplier) string {
	minimum, maximum, comma := multiplier.Min, multiplier.Max, multiplier.Comma

	if minimum == 0 && maximum == Unbounded {
		if comma {
			return "#?"
		}
		return "*"
	}

	if minimum == 0 && maximum == 1 {
		return "?"
	}

	if minimum == 1 && maximum == Unbounded {
		if comma {
			return "#"
		}
		return "+"
	}

	if minimum == 1 && maximum == 1 {
		return ""
	}

	prefix := ""
	if comma {
		prefix = "#"
	}

	if minimum == maximum {
		return prefix + "{" + strconv.Itoa(minimum) + "}"
	}

	maxText := ""
	if maximum != Unbounded {
		maxText = strconv.Itoa(maximum)
	}
	return prefix + "{" + strconv.Itoa(minimum) + "," + maxText + "}"
}

// generateTypeOpts takes a *Range, the only node type upstream accepts.
func generateTypeOpts(node *Range) string {
	minimum := "-∞"
	if node.Min != nil {
		minimum = node.Min.String()
	}

	maximum := "∞"
	if node.Max != nil {
		maximum = node.Max.String()
	}

	return " [" + minimum + "," + maximum + "]"
}

func (o GenerateOptions) generateSequence(node *Group) string {
	combinator := " " + string(node.Combinator) + " "
	if node.Combinator == Juxtaposition || o.Compact {
		combinator = string(node.Combinator)
	}

	terms := make([]string, 0, len(node.Terms))
	for _, term := range node.Terms {
		terms = append(terms, o.generate(term))
	}
	result := strings.Join(terms, combinator)

	if node.Explicit || o.ForceBraces {
		open := "[ "
		if o.Compact || strings.HasPrefix(result, ",") {
			open = "["
		}

		closing := " ]"
		if o.Compact {
			closing = "]"
		}

		return open + result + closing
	}

	return result
}

// generate is upstream internalGenerate.
func (o GenerateOptions) generate(node Node) string {
	var result string

	switch node := node.(type) {
	case *Group:
		result = o.generateSequence(node)
		if node.DisallowEmpty {
			result += "!"
		}

	case *Multiplier:
		// return since node is a composition
		return o.generate(node.Term) + o.decorate(generateMultiplier(node), node)

	case *Boolean:
		result = "<boolean-expr[" + o.generate(node.Term) + "]>"

	case *Type:
		opts := ""
		if node.Opts != nil {
			opts = o.decorate(generateTypeOpts(node.Opts), node.Opts)
		}
		result = "<" + node.Name + opts + ">"

	case *Property:
		result = "<'" + node.Name + "'>"

	case *Keyword:
		result = node.Name

	case *AtKeyword:
		result = "@" + node.Name

	case *Function:
		result = node.Name + "("

	case *String:
		result = node.Value

	case *Token:
		result = node.Value

	case *Comma:
		result = ","

	default:
		// upstream throws "Unknown node type"; Node is sealed, so only a
		// nil node (or a *Range, which upstream also rejects) reaches here
		return ""
	}

	return o.decorate(result, node)
}

// Generate returns the definition syntax text for node. A nil node (or a
// *Range on its own) generates the empty string.
func Generate(node Node, options GenerateOptions) string {
	return options.generate(node)
}
