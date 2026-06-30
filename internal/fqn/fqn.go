// Package fqn builds OpenMetadata fully-qualified names. An FQN joins hierarchy
// parts with dots; any part that itself contains a dot is wrapped in double quotes
// so the separator stays unambiguous.
package fqn

import "strings"

// Build joins the given parts into an OpenMetadata FQN, quoting any part that
// contains a dot. Empty parts are skipped. Each part must be a single name
// component, never an already-joined FQN — to extend an existing FQN with one
// more level, use Append (Build would re-quote the whole parent as one part).
func Build(parts ...string) string {
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		quoted = append(quoted, quote(p))
	}
	return strings.Join(quoted, ".")
}

// Append extends an already-built FQN with one more name component, quoting only
// the new part if it contains a dot. The parent FQN is left untouched (its parts
// are already correctly quoted), unlike Build which treats each argument as a
// single component and would wrap a dotted parent FQN in quotes.
func Append(parentFQN, part string) string {
	if part == "" {
		return parentFQN
	}
	if parentFQN == "" {
		return quote(part)
	}
	return parentFQN + "." + quote(part)
}

// quote wraps a single FQN name component in double quotes if it contains the
// dot separator, so the part stays unambiguous within the joined FQN.
func quote(part string) string {
	if strings.Contains(part, ".") {
		return `"` + part + `"`
	}
	return part
}
