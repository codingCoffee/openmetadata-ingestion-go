// Package sink defines where extracted metadata is written. The Sink interface is
// SDK-free so the workflow can be tested against a fake; the metadata-rest
// implementation translates the model into OpenMetadata SDK requests.
package sink

import (
	"context"

	"github.com/codingcoffee/openmetadata-ingestion-go/internal/model"
)

// Sink persists the metadata hierarchy. Each method upserts one entity and returns
// the fully-qualified name of the entity it created/updated so the caller can pass
// it as the parent reference for the next level down.
type Sink interface {
	UpsertService(ctx context.Context, svc *model.Service) (serviceFQN string, err error)
	UpsertDatabase(ctx context.Context, serviceFQN string, db *model.Database) (dbFQN string, err error)
	UpsertSchema(ctx context.Context, dbFQN string, schema *model.Schema) (schemaFQN string, err error)
	UpsertTable(ctx context.Context, schemaFQN string, table *model.Table) (tableFQN string, err error)
	// Close flushes and releases resources.
	Close() error
}
