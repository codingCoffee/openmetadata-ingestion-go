// Package filter implements include/exclude regex filtering for schema and table
// names, matching the semantics of the Python openmetadata-ingestion connectors.
package filter

import (
	"fmt"
	"regexp"
)

// Pattern holds raw include/exclude regular expression strings as read from the
// workflow YAML (schemaFilterPattern / tableFilterPattern).
type Pattern struct {
	Includes []string
	Excludes []string
}

// Filter is a compiled Pattern ready for matching.
type Filter struct {
	includes []*regexp.Regexp
	excludes []*regexp.Regexp
}

// New compiles a Pattern into a Filter. An invalid regular expression is returned
// as an error so misconfiguration fails fast at startup rather than mid-ingestion.
func New(p Pattern) (*Filter, error) {
	f := &Filter{}
	for _, raw := range p.Includes {
		re, err := regexp.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid include pattern %q: %w", raw, err)
		}
		f.includes = append(f.includes, re)
	}
	for _, raw := range p.Excludes {
		re, err := regexp.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", raw, err)
		}
		f.excludes = append(f.excludes, re)
	}
	return f, nil
}

// Allowed reports whether name passes the filter. Precedence matches the Python
// connectors: an exclude match rejects immediately; then if there are no includes
// everything is allowed; otherwise the name must match at least one include.
// Matching uses unanchored search (regexp.MatchString), mirroring Python's re.search.
func (f *Filter) Allowed(name string) bool {
	for _, re := range f.excludes {
		if re.MatchString(name) {
			return false
		}
	}
	if len(f.includes) == 0 {
		return true
	}
	for _, re := range f.includes {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}
