package filter

import "testing"

func TestAllowed(t *testing.T) {
	tests := []struct {
		name    string
		pattern Pattern
		input   string
		want    bool
	}{
		{"no patterns allows all", Pattern{}, "anything", true},
		{"include match", Pattern{Includes: []string{"^public$"}}, "public", true},
		{"include no match", Pattern{Includes: []string{"^public$"}}, "private", false},
		{"exclude wins over include", Pattern{Includes: []string{"data"}, Excludes: []string{"_tmp"}}, "data_tmp", false},
		{"exclude only", Pattern{Excludes: []string{"^pg_"}}, "pg_catalog", false},
		{"exclude only, non-match allowed", Pattern{Excludes: []string{"^pg_"}}, "public", true},
		{"unanchored search semantics", Pattern{Includes: []string{"foo"}}, "barfoobaz", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := New(tc.pattern)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if got := f.Allowed(tc.input); got != tc.want {
				t.Errorf("Allowed(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestNewInvalidRegex(t *testing.T) {
	if _, err := New(Pattern{Includes: []string{"("}}); err == nil {
		t.Fatal("expected error for invalid include regex")
	}
	if _, err := New(Pattern{Excludes: []string{"["}}); err == nil {
		t.Fatal("expected error for invalid exclude regex")
	}
}
