// Ported from lib/utils/names.js.

package csstree

import "strings"

// Upstream memoizes descriptors in two unbounded Maps keyed by every name it
// has seen; this port computes them on each call to avoid package-level state.

const hyphenMinus = '-' // '-'.charCodeAt()

// KeywordDescriptor is the result of Keyword. Field order and JSON keys
// match upstream.
type KeywordDescriptor struct {
	Basename string `json:"basename"`
	Name     string `json:"name"`
	Prefix   string `json:"prefix"`
	Vendor   string `json:"vendor"`
	Custom   bool   `json:"custom"`
}

// PropertyDescriptor is the result of Property. Field order and JSON keys
// match upstream.
type PropertyDescriptor struct {
	Basename string `json:"basename"`
	Name     string `json:"name"`
	Hack     string `json:"hack"`
	Vendor   string `json:"vendor"`
	Prefix   string `json:"prefix"`
	Custom   bool   `json:"custom"`
}

// Keyword describes an identifier: its lower-cased name split into vendor
// prefix and basename. Custom property names (--*) get no vendor prefix.
func Keyword(keyword string) KeywordDescriptor {
	return getKeywordDescriptor(keyword)
}

// Property describes a property name: a leading browser hack (one of
// _ * $ # + & / //), vendor prefix and basename. Names other than custom
// properties are lower-cased.
func Property(property string) PropertyDescriptor {
	return getPropertyDescriptor(property)
}

// VendorPrefix returns the vendor prefix (such as "-moz-") of str[offset:],
// or "" when it has none.
func VendorPrefix(str string, offset int) string {
	return getVendorPrefix(str, offset)
}

// IsCustomProperty reports whether str[offset:] starts with "--".
func IsCustomProperty(str string, offset int) bool {
	return len(str)-offset >= 2 &&
		str[offset] == hyphenMinus &&
		str[offset+1] == hyphenMinus
}

func getVendorPrefix(str string, offset int) string {
	// verdor prefix should be at least 3 chars length
	if len(str)-offset >= 3 {
		// vendor prefix starts with hyper minus following non-hyper minus
		if str[offset] == hyphenMinus &&
			str[offset+1] != hyphenMinus {
			// vendor prefix should contain a hyper minus at the ending
			secondDashIndex := strings.IndexByte(str[offset+2:], '-')

			if secondDashIndex != -1 {
				secondDashIndex += offset + 2
				return str[offset : secondDashIndex+1]
			}
		}
	}

	return ""
}

func getKeywordDescriptor(keyword string) KeywordDescriptor {
	name := strings.ToLower(keyword)

	custom := IsCustomProperty(name, 0)
	vendor := ""
	if !custom {
		vendor = getVendorPrefix(name, 0)
	}
	return KeywordDescriptor{
		Basename: name[len(vendor):],
		Name:     name,
		Prefix:   vendor,
		Vendor:   vendor,
		Custom:   custom,
	}
}

func getPropertyDescriptor(property string) PropertyDescriptor {
	name := property
	hack := ""
	if len(property) > 0 {
		hack = property[:1]
	}

	if hack == "/" {
		if len(property) > 1 && property[1] == '/' {
			hack = "//"
		} else {
			hack = "/"
		}
	} else if hack != "_" &&
		hack != "*" &&
		hack != "$" &&
		hack != "#" &&
		hack != "+" &&
		hack != "&" {
		hack = ""
	}

	custom := IsCustomProperty(name, len(hack))

	if !custom {
		name = strings.ToLower(name)
	}

	vendor := ""
	if !custom {
		vendor = getVendorPrefix(name, len(hack))
	}
	prefix := name[:len(hack)+len(vendor)]
	return PropertyDescriptor{
		Basename: name[len(prefix):],
		Name:     name[len(hack):],
		Hack:     hack,
		Vendor:   vendor,
		Prefix:   prefix,
		Custom:   custom,
	}
}
