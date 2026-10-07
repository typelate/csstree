// Ported from lib/lexer/generic-const.js.

package csstree

// cssWideKeywords returns the CSS-wide keywords.
//
// https://drafts.csswg.org/css-cascade-5/
func cssWideKeywords() []string {
	return []string{
		"initial",
		"inherit",
		"unset",
		"revert",
		"revert-layer",
	}
}

// isCSSWideKeyword reports whether name, which is lowercase, is one of
// cssWideKeywords.
func isCSSWideKeyword(name string) bool {
	switch name {
	case "initial", "inherit", "unset", "revert", "revert-layer":
		return true
	}
	return false
}
