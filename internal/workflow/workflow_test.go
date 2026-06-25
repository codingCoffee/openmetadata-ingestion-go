package workflow

import (
	"context"
	"testing"

	"github.com/zerodha/openmetadata-ingestion-go/internal/model"
)

// fakeSource returns a fixed service tree.
type fakeSource struct{ svc *model.Service }

func (f *fakeSource) Prepare(context.Context) error                  { return nil }
func (f *fakeSource) Extract(context.Context) (*model.Service, error) { return f.svc, nil }
func (f *fakeSource) Close() error                                   { return nil }

// fakeSink records the order of upserts and the parent FQNs passed in.
type fakeSink struct {
	calls       []string
	tableParent string
}

func (s *fakeSink) UpsertService(_ context.Context, svc *model.Service) (string, error) {
	s.calls = append(s.calls, "service:"+svc.Name)
	return svc.Name, nil
}
func (s *fakeSink) UpsertDatabase(_ context.Context, serviceFQN string, db *model.Database) (string, error) {
	s.calls = append(s.calls, "database:"+db.Name)
	return serviceFQN + "." + db.Name, nil
}
func (s *fakeSink) UpsertSchema(_ context.Context, dbFQN string, sc *model.Schema) (string, error) {
	s.calls = append(s.calls, "schema:"+sc.Name)
	return dbFQN + "." + sc.Name, nil
}
func (s *fakeSink) UpsertTable(_ context.Context, schemaFQN string, tbl *model.Table) (string, error) {
	s.calls = append(s.calls, "table:"+tbl.Name)
	s.tableParent = schemaFQN
	return schemaFQN + "." + tbl.Name, nil
}
func (s *fakeSink) Close() error { return nil }

func TestRunOrderAndFQNPropagation(t *testing.T) {
	svc := &model.Service{
		Name:        "svc",
		ServiceType: "Postgres",
		Databases: []*model.Database{{
			Name: "db",
			Schemas: []*model.Schema{{
				Name:   "public",
				Tables: []*model.Table{{Name: "t1"}},
			}},
		}},
	}
	src := &fakeSource{svc: svc}
	snk := &fakeSink{}

	stats, err := New(src, snk, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"service:svc", "database:db", "schema:public", "table:t1"}
	if len(snk.calls) != len(want) {
		t.Fatalf("calls = %v, want %v", snk.calls, want)
	}
	for i := range want {
		if snk.calls[i] != want[i] {
			t.Errorf("call[%d] = %q, want %q", i, snk.calls[i], want[i])
		}
	}
	if snk.tableParent != "svc.db.public" {
		t.Errorf("table parent FQN = %q, want %q", snk.tableParent, "svc.db.public")
	}
	if stats.Databases != 1 || stats.Schemas != 1 || stats.Tables != 1 {
		t.Errorf("stats = %+v", stats)
	}
}
