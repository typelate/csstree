package csstree

import "testing"

// TestIsCSSWideKeyword checks that isCSSWideKeyword knows the keywords
// cssWideKeywords lists, and only those.
func TestIsCSSWideKeyword(t *testing.T) {
	for _, keyword := range cssWideKeywords() {
		if !isCSSWideKeyword(keyword) {
			t.Errorf("isCSSWideKeyword(%q) = false, want true", keyword)
		}
	}
	for _, name := range []string{"default", "Initial", "auto", ""} {
		if isCSSWideKeyword(name) {
			t.Errorf("isCSSWideKeyword(%q) = true, want false", name)
		}
	}
}
