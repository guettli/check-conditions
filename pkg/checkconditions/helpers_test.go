package checkconditions

import (
	"strings"
	"testing"
)

func TestMatchAnyPattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		patterns []string
		want     bool
	}{
		{"exact match", "kube-system", []string{"kube-system"}, true},
		{"no patterns", "kube-system", nil, false},
		{"glob star match", "kube-system", []string{"kube-*"}, true},
		{"glob no match", "default", []string{"kube-*"}, false},
		{"second pattern matches", "default", []string{"kube-*", "default"}, true},
		{"question mark glob", "foo1", []string{"foo?"}, true},
		{"char class glob", "foo2", []string{"foo[0-9]"}, true},
		{"invalid pattern is ignored, not matched", "anything", []string{"[invalid"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchAnyPattern(tt.input, tt.patterns); got != tt.want {
				t.Errorf("matchAnyPattern(%q, %v) = %v, want %v", tt.input, tt.patterns, got, tt.want)
			}
		})
	}
}

func TestValidatePatterns(t *testing.T) {
	if err := validatePatterns([]string{"kube-*", "default", "foo[0-9]"}); err != nil {
		t.Errorf("expected valid patterns to pass, got: %v", err)
	}
	if err := validatePatterns(nil); err != nil {
		t.Errorf("expected nil patterns to pass, got: %v", err)
	}
	err := validatePatterns([]string{"good", "[bad"})
	if err == nil {
		t.Fatal("expected an error for an invalid glob pattern, got nil")
	}
	// The offending pattern should be named in the error to help the user.
	if want := "[bad"; !strings.Contains(err.Error(), want) {
		t.Errorf("expected error to mention %q, got: %v", want, err)
	}
}

func TestPatternHasGlob(t *testing.T) {
	tests := map[string]bool{
		"plain":    false,
		"kube-*":   true,
		"foo?":     true,
		"foo[0-9]": true,
		"":         false,
	}
	for in, want := range tests {
		if got := patternHasGlob(in); got != want {
			t.Errorf("patternHasGlob(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestContainsSlash(t *testing.T) {
	// containsSlash reports whether the string starts with a slash.
	tests := map[string]bool{
		"/api": true,
		"api":  false,
		"":     false,
		"a/b":  false,
		"/":    true,
	}
	for in, want := range tests {
		if got := containsSlash(in); got != want {
			t.Errorf("containsSlash(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNamespaceSet(t *testing.T) {
	if got := (&Arguments{}).namespaceSet(); got != nil {
		t.Errorf("expected nil set when no namespaces are resolved, got %v", got)
	}
	a := &Arguments{Namespaces: []string{"a", "b", "a"}}
	set := a.namespaceSet()
	if len(set) != 2 {
		t.Fatalf("expected 2 unique namespaces, got %d: %v", len(set), set)
	}
	for _, ns := range []string{"a", "b"} {
		if _, ok := set[ns]; !ok {
			t.Errorf("expected namespace %q in set", ns)
		}
	}
	if _, ok := set["c"]; ok {
		t.Error("did not expect namespace \"c\" in set")
	}
}

func TestNamespaceFilterActive(t *testing.T) {
	if (&Arguments{}).namespaceFilterActive() {
		t.Error("expected no filter active with empty NamespacePatterns")
	}
	if !(&Arguments{NamespacePatterns: []string{"kube-*"}}).namespaceFilterActive() {
		t.Error("expected filter active when NamespacePatterns is set")
	}
	// Resolved Namespaces alone should not count as an active user filter.
	if (&Arguments{Namespaces: []string{"kube-system"}}).namespaceFilterActive() {
		t.Error("resolved Namespaces without NamespacePatterns should not report an active filter")
	}
}

func TestCounterAdd(t *testing.T) {
	c := &Counter{}
	c.add(handleResourceTypeOutput{
		checkedResourceTypes: 1,
		checkedResources:     2,
		checkedConditions:    3,
		lines:                []string{"line-a"},
		forbiddenResource:    "secrets",
	})
	c.add(handleResourceTypeOutput{
		checkedResourceTypes: 1,
		checkedResources:     1,
		checkedConditions:    4,
		lines:                []string{"line-b"},
		whileRegexDidMatch:   true,
	})
	// A later output that did NOT match must not clear the flag: it is sticky.
	c.add(handleResourceTypeOutput{whileRegexDidMatch: false})

	if c.CheckedResourceTypes != 2 || c.CheckedResources != 3 || c.CheckedConditions != 7 {
		t.Errorf("unexpected counts: types=%d resources=%d conditions=%d",
			c.CheckedResourceTypes, c.CheckedResources, c.CheckedConditions)
	}
	if len(c.Lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %v", len(c.Lines), c.Lines)
	}
	if len(c.ForbiddenResources) != 1 || c.ForbiddenResources[0] != "secrets" {
		t.Errorf("expected one forbidden resource \"secrets\", got %v", c.ForbiddenResources)
	}
	if !c.WhileRegexDidMatch {
		t.Error("expected WhileRegexDidMatch to be sticky once any output set it")
	}
}
