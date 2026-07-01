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
		{"escapes embedded quote", []string{"svc", `a.b"c`, "t"}, `svc."a.b""c".t`},
		{"quotes part with only quote", []string{"svc", `a"b`, "t"}, `svc."a""b".t`},
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

func TestAppend(t *testing.T) {
	tests := []struct {
		name      string
		parentFQN string
		part      string
		want      string
	}{
		// The parent FQN already contains dots; Append must not re-quote it.
		{"extends dotted parent", "local_postgres.postgres", "public", "local_postgres.postgres.public"},
		{"chained levels", "local_postgres.postgres.public", "active_customers", "local_postgres.postgres.public.active_customers"},
		{"quotes only new dotted part", "svc.db", "my.schema", `svc.db."my.schema"`},
		{"empty parent", "", "svc", "svc"},
		{"empty part returns parent", "svc.db", "", "svc.db"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Append(tc.parentFQN, tc.part); got != tc.want {
				t.Errorf("Append(%q, %q) = %q, want %q", tc.parentFQN, tc.part, got, tc.want)
			}
		})
	}
}
