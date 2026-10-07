// Ported from lib/syntax/config/mix.js.

package csstree

import (
	"maps"
	"slices"
	"strings"
)

// jsWhiteSpace is the ASCII part of the JavaScript regexp class \s.
const jsWhiteSpace = " \t\n\v\f\r"

// appendOrSet returns a + b when b starts with "|" (after whitespace) and
// a is set, b without that "|" when a is not set, and b otherwise. "" is
// upstream null.
func appendOrSet(a, b string) string {
	if trimmed := strings.TrimLeft(b, jsWhiteSpace); strings.HasPrefix(trimmed, "|") {
		if a != "" {
			return a + b
		}
		return strings.TrimLeft(trimmed[1:], jsWhiteSpace)
	}

	return b
}

// mergeRecognizer is upstream's { ...base, ...ext } for a scope: fields of
// ext that are set replace those of base, and Functions are merged by
// name.
func mergeRecognizer(base, ext *Recognizer) *Recognizer {
	if ext == nil {
		ext = new(Recognizer)
	}
	result := new(Recognizer)
	if base != nil {
		*result = *base
		result.Functions = maps.Clone(base.Functions)
	}

	if ext.Node != nil {
		result.Node = ext.Node
	}
	if ext.OnWhiteSpace != nil {
		result.OnWhiteSpace = ext.OnWhiteSpace
	}
	if len(ext.Functions) > 0 {
		if result.Functions == nil {
			result.Functions = make(map[string]FunctionParser)
		}
		maps.Copy(result.Functions, ext.Functions)
	}

	return result
}

// mergeNodeConfig is upstream's { ...base, ...extractProps(ext, ['name',
// 'structure', 'parse', 'generate', 'walkContext']) }: fields of ext that
// are set replace those of base. The Go-only Walk, Clone and
// FromPlainObject are merged the same way.
func mergeNodeConfig(base, ext NodeConfig) NodeConfig {
	result := base

	if ext.Name != "" {
		result.Name = ext.Name
	}
	if ext.Structure != nil {
		result.Structure = ext.Structure
	}
	if ext.Generate != nil {
		result.Generate = ext.Generate
	}
	if ext.WalkContext != "" {
		result.WalkContext = ext.WalkContext
	}
	if ext.Walk != nil {
		result.Walk = ext.Walk
	}
	if ext.Clone != nil {
		result.Clone = ext.Clone
	}
	if ext.FromPlainObject != nil {
		result.FromPlainObject = ext.FromPlainObject
	}

	return result
}

// mix returns a new config: dest with the keys set in src mixed in.
// Upstream copies src keys that are present; here a key is present when
// its field is not the zero value (nil map, nil slice, nil Generic). The
// maps of the keys set in src are new; the others are shared with dest.
func mix(dest, src *Config) *Config {
	if dest == nil {
		dest = new(Config)
	}
	if src == nil {
		src = new(Config)
	}
	result := *dest

	if src.Generic != nil {
		result.Generic = new(*src.Generic)
	}

	if src.CSSWideKeywords != nil {
		if dest.CSSWideKeywords != nil {
			result.CSSWideKeywords = slices.Concat(dest.CSSWideKeywords, src.CSSWideKeywords)
		} else {
			result.CSSWideKeywords = slices.Clone(src.CSSWideKeywords)
		}
	}

	result.Units = overlay(dest.Units, src.Units, func(_, patch []string) []string {
		if patch == nil {
			return []string{}
		}
		return patch
	})
	result.Atrules = overlay(dest.Atrules, src.Atrules, mixAtrule)
	result.Types = overlay(dest.Types, src.Types, appendOrSet)
	result.Properties = overlay(dest.Properties, src.Properties, appendOrSet)

	result.Contexts = overlay(dest.Contexts, src.Contexts, replace)
	result.Scope = overlay(dest.Scope, src.Scope, mergeRecognizer)
	result.Features = overlay(dest.Features, src.Features, func(base, ext map[string]FeatureParser) map[string]FeatureParser {
		merged := make(map[string]FeatureParser, len(base)+len(ext))
		maps.Copy(merged, base)
		maps.Copy(merged, ext)
		return merged
	})
	result.Atrule = overlay(dest.Atrule, src.Atrule, replace)
	result.Pseudo = overlay(dest.Pseudo, src.Pseudo, replace)
	result.Node = overlay(dest.Node, src.Node, mergeNodeConfig)

	return &result
}

// overlay returns a new map of the entries of dest and src, merging the
// value of a key of src with merge(dest value or zero value, src value);
// it returns dest itself when src is nil (upstream: the key is absent).
func overlay[K comparable, V any](dest, src map[K]V, merge func(base, ext V) V) map[K]V {
	if src == nil {
		return dest
	}

	result := make(map[K]V, len(dest)+len(src))
	maps.Copy(result, dest)
	for key, value := range src {
		result[key] = merge(result[key], value)
	}

	return result
}

// replace is the merge of overlay that keeps the value of src.
func replace[V any](_, ext V) V {
	return ext
}

// mixAtrule is the atrules case of mix: the prelude and descriptor
// syntaxes of atrule are mixed into exists with appendOrSet, and a
// descriptor syntax of "" (upstream null) is set as is.
func mixAtrule(exists, atrule AtruleDefinition) AtruleDefinition {
	current := AtruleDefinition{
		Prelude:     exists.Prelude,
		Descriptors: maps.Clone(exists.Descriptors),
	}
	if current.Descriptors == nil {
		current.Descriptors = make(map[string]string)
	}

	if atrule.Prelude != "" {
		current.Prelude = appendOrSet(current.Prelude, atrule.Prelude)
	}

	for descriptorName, descriptorValue := range atrule.Descriptors {
		if descriptorValue != "" {
			current.Descriptors[descriptorName] = appendOrSet(current.Descriptors[descriptorName], descriptorValue)
		} else {
			current.Descriptors[descriptorName] = ""
		}
	}

	if len(current.Descriptors) == 0 {
		current.Descriptors = nil
	}

	return current
}
