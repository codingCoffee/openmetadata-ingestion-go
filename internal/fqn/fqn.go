// Package fqn builds OpenMetadata fully-qualified names. An FQN joins hierarchy
// parts with dots; any part that itself contains a dot is wrapped in double quotes
// so the separator stays unambiguous.
package fqn

import "strings"

// Build joins the given parts into an OpenMetadata FQN, quoting any part that
// contains a dot. Empty parts are skipped.
func Build(parts ...string) string {
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.Contains(p, ".") {
			p = `"` + p + `"`
		}
		quoted = append(quoted, p)
	}
	return strings.Join(quoted, ".")
}
