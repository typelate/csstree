// Ported from lib/lexer/structure.js.

package csstree

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/typelate/csstree/internal/jsnum"
)

// StructureWarning is a problem CheckStructure found in a node.
type StructureWarning struct {
	// Node is the node's plain object form (see ToPlainObject).
	Node any
	// Message describes the problem.
	Message string
}

// nodeStructure is the checker of a node type (upstream
// processStructure's result): docs describes the type of each field.
type nodeStructure struct {
	docs  map[string]string
	check func(node any, warn func(node any, message string))
}

// checkerField is an entry of the fields createNodeStructureChecker
// checks: "type", "loc" (upstream `loc: true`) and the structure fields.
type checkerField struct {
	name  string
	types []FieldType
}

func isValidNumber(value any) bool {
	// Number.isInteger(value) && value >= 0
	switch value := value.(type) {
	case float64:
		return !math.IsInf(value, 0) &&
			!math.IsNaN(value) &&
			math.Floor(value) == value &&
			value >= 0
	case int:
		return value >= 0
	}

	return false
}

func isValidLocation(loc any) bool {
	object, ok := loc.(map[string]any)

	return ok && object != nil &&
		isValidNumber(object["offset"]) &&
		isValidNumber(object["line"]) &&
		isValidNumber(object["column"])
}

// nodeKeys returns the keys of node in the order upstream's for-in loop
// visits them in a parsed AST: type, loc, the structure fields in order,
// then any other keys sorted (a Go map has no insertion order).
func nodeKeys(node map[string]any, fields []checkerField) []string {
	keys := make([]string, 0, len(node))
	known := make(map[string]bool, len(fields))

	for _, field := range fields {
		known[field.name] = true
		if _, ok := node[field.name]; ok {
			keys = append(keys, field.name)
		}
	}

	var rest []string
	for key := range node {
		if !known[key] {
			rest = append(rest, key)
		}
	}
	slices.Sort(rest)

	return append(keys, rest...)
}

func createNodeStructureChecker(typ string, fields []checkerField) func(node any, warn func(node any, message string)) {
	fieldTypes := make(map[string][]FieldType, len(fields))
	for _, field := range fields {
		fieldTypes[field.name] = field.types
	}

	return func(value any, warn func(node any, message string)) {
		node, ok := value.(map[string]any)
		if !ok || node == nil {
			warn(value, "Type of node should be an Object")
			return
		}

		for _, key := range nodeKeys(node, fields) {
			valid := true

			if key == "type" {
				if nodeType, ok := node["type"].(string); !ok || nodeType != typ {
					warn(node, "Wrong node type `"+jsString(node["type"])+"`, expected `"+typ+"`")
				}
			} else if key == "loc" {
				if node["loc"] == nil {
					continue
				} else if loc, ok := node["loc"].(map[string]any); ok {
					if _, ok := loc["source"].(string); !ok {
						key += ".source"
					} else if !isValidLocation(loc["start"]) {
						key += ".start"
					} else if !isValidLocation(loc["end"]) {
						key += ".end"
					} else {
						continue
					}
				}

				valid = false
			} else if types, ok := fieldTypes[key]; ok {
				valid = false

				for i := 0; !valid && i < len(types); i++ {
					fieldType := types[i]

					switch fieldType.Kind {
					case FieldString:
						_, valid = node[key].(string)

					case FieldBoolean:
						_, valid = node[key].(bool)

					case FieldNull:
						valid = node[key] == nil

					case FieldNode:
						child, ok := node[key].(map[string]any)
						valid = ok && child != nil && child["type"] == fieldType.Node

					case FieldList:
						// upstream: node[key] instanceof List; lists are
						// arrays in the plain object form
						_, valid = node[key].([]any)
					}
				}
			} else {
				warn(node, "Unknown field `"+key+"` for "+typ+" node type")
			}

			if !valid {
				warn(node, "Bad value for `"+typ+"."+key+"`")
			}
		}

		for _, field := range fields {
			if _, ok := node[field.name]; !ok {
				warn(node, "Field `"+typ+"."+field.name+"` is missed")
			}
		}
	}
}

// jsString formats a JSON value as JavaScript's string concatenation does.
func jsString(value any) string {
	switch value := value.(type) {
	case nil:
		return "null"
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case float64:
		return jsnum.Format(value)
	case int:
		return strconv.Itoa(value)
	case map[string]any:
		return "[object Object]"
	case []any:
		// Array.prototype.join: null items are empty
		items := make([]string, len(value))
		for i, item := range value {
			if item != nil {
				items[i] = jsString(item)
			}
		}
		return strings.Join(items, ",")
	}
	return fmt.Sprint(value)
}

func genTypesList(fieldTypes []FieldType, path string) (string, error) {
	var docsTypes []string

	for i := 0; i < len(fieldTypes); i++ {
		fieldType := fieldTypes[i]
		switch fieldType.Kind {
		case FieldString:
			docsTypes = append(docsTypes, "string")
		case FieldBoolean:
			docsTypes = append(docsTypes, "boolean")
		case FieldNull:
			docsTypes = append(docsTypes, "null")
		case FieldNode:
			docsTypes = append(docsTypes, fieldType.Node)
		case FieldList:
			listTypes := make([]FieldType, len(fieldType.List))
			for j, nodeType := range fieldType.List {
				listTypes[j] = nodeField(nodeType)
			}
			list, err := genTypesList(listTypes, path)
			if err != nil {
				return "", err
			}
			if list == "" {
				list = "any"
			}
			docsTypes = append(docsTypes, "List<"+list+">") // TODO: use type enum
		default:
			//lint:ignore ST1005 upstream message
			return "", fmt.Errorf("Wrong value `%d` in `%s` structure definition", fieldType.Kind, path)
		}
	}

	return strings.Join(docsTypes, " | "), nil
}

func processStructure(name string, nodeType NodeConfig) (*nodeStructure, error) {
	structure := nodeType.Structure
	fields := []checkerField{
		{name: "type", types: []FieldType{stringField()}},
		{name: "loc"},
	}
	docs := map[string]string{
		"type": `"` + name + `"`,
	}

	for _, field := range structure {
		fieldTypes := slices.Clone(field.Types)
		fields = append(fields, checkerField{name: field.Name, types: fieldTypes})

		typesList, err := genTypesList(fieldTypes, name+"."+field.Name)
		if err != nil {
			return nil, err
		}
		docs[field.Name] = typesList
	}

	return &nodeStructure{
		docs:  docs,
		check: createNodeStructureChecker(name, fields),
	}, nil
}

func getStructureFromConfig(config *Config) (map[string]*nodeStructure, error) {
	structure := make(map[string]*nodeStructure)
	if config == nil {
		return structure, nil
	}

	for _, name := range sortedKeys(config.Node) {
		nodeType := config.Node[name]
		if nodeType.Structure == nil {
			// upstream "Missed `structure` field in `name` node type
			// definition"
			return nil, &MissingStructureError{NodeType: name}
		}

		processed, err := processStructure(name, nodeType)
		if err != nil {
			return nil, err
		}
		structure[name] = processed
	}

	return structure, nil
}
