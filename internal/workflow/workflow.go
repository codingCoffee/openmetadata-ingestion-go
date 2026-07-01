// Package workflow wires a Source to a Sink and drives the metadata push in the
// order required by OpenMetadata's entity hierarchy: service, then databases,
// then schemas, then tables. Every level is upserted, so re-runs are idempotent.
package workflow

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/codingcoffee/openmetadata-ingestion-go/internal/sink"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/source"
)

// Workflow couples a source and a sink.
type Workflow struct {
	src  source.Source
	sink sink.Sink
	log  *slog.Logger
}

// New builds a Workflow.
func New(src source.Source, snk sink.Sink, log *slog.Logger) *Workflow {
	if log == nil {
		log = slog.Default()
	}
	return &Workflow{src: src, sink: snk, log: log}
}

// Stats summarises what a run pushed.
type Stats struct {
	Databases int
	Schemas   int
	Tables    int
	// FailedTables counts tables skipped because their upsert failed. The run
	// continues past such failures rather than aborting.
	FailedTables int
}

// Run extracts from the source and pushes the full hierarchy to the sink.
func (w *Workflow) Run(ctx context.Context) (Stats, error) {
	var stats Stats

	if err := w.src.Prepare(ctx); err != nil {
		return stats, fmt.Errorf("preparing source: %w", err)
	}
	defer w.src.Close()
	defer w.sink.Close()

	svc, err := w.src.Extract(ctx)
	if err != nil {
		return stats, fmt.Errorf("extracting metadata: %w", err)
	}

	serviceFQN, err := w.sink.UpsertService(ctx, svc)
	if err != nil {
		return stats, fmt.Errorf("upserting service %q: %w", svc.Name, err)
	}
	w.log.Info("upserted service", "service", serviceFQN)

	for _, db := range svc.Databases {
		dbFQN, err := w.sink.UpsertDatabase(ctx, serviceFQN, db)
		if err != nil {
			return stats, fmt.Errorf("upserting database %q: %w", db.Name, err)
		}
		stats.Databases++
		w.log.Debug("upserted database", "database", dbFQN)

		for _, schema := range db.Schemas {
			schemaFQN, err := w.sink.UpsertSchema(ctx, dbFQN, schema)
			if err != nil {
				return stats, fmt.Errorf("upserting schema %q: %w", schema.Name, err)
			}
			stats.Schemas++
			w.log.Debug("upserted schema", "schema", schemaFQN)

			var upserted int
			for _, table := range schema.Tables {
				if _, err := w.sink.UpsertTable(ctx, schemaFQN, table); err != nil {
					// A single table failing (e.g. an unsupported column type the
					// server rejects) must not abort the whole run: log it and move on.
					stats.FailedTables++
					w.log.Error("skipping table", "table", table.Name, "schema", schemaFQN, "err", err)
					continue
				}
				stats.Tables++
				upserted++
			}
			w.log.Info("upserted schema tables", "schema", schemaFQN, "tables", upserted)
		}
	}

	return stats, nil
}
