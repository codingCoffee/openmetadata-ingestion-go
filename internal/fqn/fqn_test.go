package fqn

import "testing"

func TestBuild(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"simple", []string{"svc", "db", "schema", "table"}, "svc.db.schema.table"},
		{"empty parts skipped", []string{"svc", "", "schema"}, "svc.schema"},
		{"quotes dotted part", []string{"svc", "my.db", "t"}, `svc."my.db".t`},
		{"single", []string{"svc"}, "svc"},
		{"none", nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Build(tc.parts...); got != tc.want {
				t.Errorf("Build(%q) = %q, want %q", tc.parts, got, tc.want)
			}
		})
	}
}
