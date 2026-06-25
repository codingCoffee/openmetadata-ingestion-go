package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadPostgresTestdata(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "workflow.postgres.yaml")
	wf, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if wf.Source.Type != "postgres" {
		t.Errorf("source.type = %q", wf.Source.Type)
	}
	conn, ok := wf.Source.ServiceConnection.Config.(*PostgresConnection)
	if !ok {
		t.Fatalf("connection type = %T, want *PostgresConnection", wf.Source.ServiceConnection.Config)
	}
	if conn.ConnType() != "Postgres" {
		t.Errorf("ConnType = %q", conn.ConnType())
	}
	if conn.SSLMode != "disable" {
		t.Errorf("sslMode = %q", conn.SSLMode)
	}
	if conn.Base().Username != "postgres" {
		t.Errorf("username = %q", conn.Base().Username)
	}
	sc := wf.Source.SourceConfig.Config
	if !sc.IncludeViewsOrDefault() || !sc.IncludeTablesOrDefault() {
		t.Error("expected includeTables/includeViews true")
	}
	if got := sc.SchemaFilterPattern.Includes; len(got) != 1 || got[0] != "public" {
		t.Errorf("schema includes = %v", got)
	}
	if wf.WorkflowConfig.OpenMetadataServerConfig.SecurityConfig.JWTToken == "" {
		t.Error("expected jwtToken to be set")
	}
}

func TestPolymorphicDecode(t *testing.T) {
	cases := map[string]string{
		"PostgreSQL": "*config.PostgresConnection",
		"Mysql":      "*config.MySQLConnection",
		"Clickhouse": "*config.ClickHouseConnection",
	}
	for typ := range cases {
		var sc ServiceConnection
		doc := "config:\n  type: " + typ + "\n  hostPort: h:1\n  username: u\n"
		if err := yaml.Unmarshal([]byte(doc), &sc); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if sc.Config == nil {
			t.Fatalf("%s: nil config", typ)
		}
	}
}

func TestUnknownFieldsTolerated(t *testing.T) {
	doc := `
source:
  type: postgres
  serviceName: s
  serviceConnection:
    config:
      type: PostgreSQL
      hostPort: h:5432
      username: u
      someFutureField: ignored
  sourceConfig:
    config:
      type: DatabaseMetadata
      anotherUnknown: 42
sink:
  type: metadata-rest
  config: {}
workflowConfig:
  openMetadataServerConfig:
    hostPort: http://h/api
    securityConfig:
      jwtToken: tok
`
	f := filepath.Join(t.TempDir(), "wf.yaml")
	if err := os.WriteFile(f, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(f); err != nil {
		t.Fatalf("Load with unknown fields: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(f, []byte("source:\n  type: postgres\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(f); err == nil {
		t.Fatal("expected validation error for incomplete config")
	}
}
