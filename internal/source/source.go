// Package source defines the Source abstraction and a registry of source
// implementations keyed by connection type. Each source connects to a database,
// extracts a source-neutral model.Service tree (already filtered), and closes.
package source

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/zerodha/openmetadata-ingestion-go/internal/config"
	"github.com/zerodha/openmetadata-ingestion-go/internal/model"
)

// Source extracts metadata from a single database service.
type Source interface {
	// Prepare opens connections and validates that extraction can proceed.
	Prepare(ctx context.Context) error
	// Extract pulls the full, filtered metadata hierarchy.
	Extract(ctx context.Context) (*model.Service, error)
	// Close releases any open resources.
	Close() error
}

// Factory builds a Source from the parsed source section of a workflow config.
type Factory func(src config.Source) (Source, error)

var registry = map[string]Factory{}

// Register associates a connection type (case-insensitive, e.g. "postgres") with a
// factory. It is intended to be called from source package init functions.
func Register(connType string, f Factory) {
	registry[strings.ToLower(connType)] = f
}

// New constructs the Source for the given config, dispatching on the connection
// type reported by the parsed serviceConnection.config.
func New(src config.Source) (Source, error) {
	if src.ServiceConnection.Config == nil {
		return nil, fmt.Errorf("no serviceConnection.config parsed")
	}
	key := strings.ToLower(src.ServiceConnection.Config.ConnType())
	f, ok := registry[key]
	if !ok {
		return nil, fmt.Errorf("no source registered for connection type %q (registered: %s)", key, strings.Join(registered(), ", "))
	}
	return f(src)
}

func registered() []string {
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
