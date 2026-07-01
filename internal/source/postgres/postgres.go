// Package postgres extracts schema metadata from a PostgreSQL service.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // register the "pgx" database/sql driver

	"github.com/codingcoffee/openmetadata-ingestion-go/internal/config"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/filter"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/model"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/source"
)

func init() {
	source.Register("postgres", New)
}

// Source extracts metadata from a PostgreSQL instance.
type Source struct {
	conn         *config.PostgresConnection
	serviceName  string
	schemaFilter *filter.Filter
	tableFilter  *filter.Filter
	includeViews bool
	includeTabls bool
}

// New builds a postgres Source from the parsed workflow source config.
func New(src config.Source) (source.Source, error) {
	conn, ok := src.ServiceConnection.Config.(*config.PostgresConnection)
	if !ok {
		return nil, fmt.Errorf("postgres source: unexpected connection type %T", src.ServiceConnection.Config)
	}
	sc := src.SourceConfig.Config
	sf, err := filter.New(filter.Pattern{Includes: sc.SchemaFilterPattern.Includes, Excludes: sc.SchemaFilterPattern.Excludes})
	if err != nil {
		return nil, fmt.Errorf("postgres source: schema filter: %w", err)
	}
	tf, err := filter.New(filter.Pattern{Includes: sc.TableFilterPattern.Includes, Excludes: sc.TableFilterPattern.Excludes})
	if err != nil {
		return nil, fmt.Errorf("postgres source: table filter: %w", err)
	}
	return &Source{
		conn:         conn,
		serviceName:  src.ServiceName,
		schemaFilter: sf,
		tableFilter:  tf,
		includeViews: sc.IncludeViewsOrDefault(),
		includeTabls: sc.IncludeTablesOrDefault(),
	}, nil
}

// Prepare validates connectivity by pinging the configured database.
func (s *Source) Prepare(ctx context.Context) error {
	target := s.conn.Database
	if target == "" {
		target = "postgres"
	}
	db, err := s.open(target)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.PingContext(ctx)
}

// Close is a no-op: connections are opened and closed per database in Extract.
func (s *Source) Close() error { return nil }

// Extract pulls databases, schemas, tables and columns.
func (s *Source) Extract(ctx context.Context) (*model.Service, error) {
	svc := &model.Service{Name: s.serviceName, ServiceType: "Postgres"}

	dbNames, err := s.databaseNames(ctx)
	if err != nil {
		return nil, err
	}

	for _, dbName := range dbNames {
		db, err := s.open(dbName)
		if err != nil {
			return nil, fmt.Errorf("connecting to database %q: %w", dbName, err)
		}
		modelDB, err := s.extractDatabase(ctx, db, dbName)
		db.Close()
		if err != nil {
			return nil, err
		}
		svc.Databases = append(svc.Databases, modelDB)
	}
	return svc, nil
}

// databaseNames returns the databases to ingest.
func (s *Source) databaseNames(ctx context.Context) ([]string, error) {
	if !s.conn.IngestAllDatabases {
		if s.conn.Database == "" {
			return nil, fmt.Errorf("postgres source: database is required when ingestAllDatabases is false")
		}
		return []string{s.conn.Database}, nil
	}
	db, err := s.open(orDefault(s.conn.Database, "postgres"))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `SELECT datname FROM pg_database WHERE datistemplate = false AND datallowconn = true`)
	if err != nil {
		return nil, fmt.Errorf("listing databases: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *Source) extractDatabase(ctx context.Context, db *sql.DB, dbName string) (*model.Database, error) {
	modelDB := &model.Database{Name: dbName}

	rows, err := db.QueryContext(ctx, `
		SELECT n.nspname, COALESCE(obj_description(n.oid, 'pg_namespace'), '')
		FROM pg_namespace n
		WHERE n.nspname NOT LIKE 'pg\_%' AND n.nspname <> 'information_schema'
		ORDER BY n.nspname`)
	if err != nil {
		return nil, fmt.Errorf("listing schemas in %q: %w", dbName, err)
	}
	defer rows.Close()

	var schemaNames []string
	descs := map[string]string{}
	for rows.Next() {
		var name, desc string
		if err := rows.Scan(&name, &desc); err != nil {
			return nil, err
		}
		if !s.schemaFilter.Allowed(name) {
			continue
		}
		schemaNames = append(schemaNames, name)
		descs[name] = desc
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, schemaName := range schemaNames {
		schema := &model.Schema{Name: schemaName, Description: descs[schemaName]}
		tables, err := s.extractTables(ctx, db, schemaName)
		if err != nil {
			return nil, err
		}
		schema.Tables = tables
		modelDB.Schemas = append(modelDB.Schemas, schema)
	}
	return modelDB, nil
}

func (s *Source) extractTables(ctx context.Context, db *sql.DB, schemaName string) ([]*model.Table, error) {
	// relkind: r=table, p=partitioned table, v=view, m=materialized view, f=foreign table.
	rows, err := db.QueryContext(ctx, `
		SELECT c.relname, c.relkind, COALESCE(obj_description(c.oid, 'pg_class'), '')
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r','p','v','m','f')
		ORDER BY c.relname`, schemaName)
	if err != nil {
		return nil, fmt.Errorf("listing tables in schema %q: %w", schemaName, err)
	}
	defer rows.Close()

	type tableMeta struct {
		name   string
		isView bool
		desc   string
	}
	var metas []tableMeta
	for rows.Next() {
		var name, kind, desc string
		if err := rows.Scan(&name, &kind, &desc); err != nil {
			return nil, err
		}
		isView := kind == "v" || kind == "m"
		if isView && !s.includeViews {
			continue
		}
		if !isView && !s.includeTabls {
			continue
		}
		if !s.tableFilter.Allowed(name) {
			continue
		}
		metas = append(metas, tableMeta{name: name, isView: isView, desc: desc})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var tables []*model.Table
	for _, m := range metas {
		cols, err := s.extractColumns(ctx, db, schemaName, m.name)
		if err != nil {
			return nil, err
		}
		pks, err := s.primaryKeys(ctx, db, schemaName, m.name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, &model.Table{
			Name:        m.name,
			Description: m.desc,
			IsView:      m.isView,
			Columns:     cols,
			PrimaryKeys: pks,
		})
	}
	return tables, nil
}

func (s *Source) extractColumns(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]*model.Column, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.attname,
		       pg_catalog.format_type(a.atttypid, a.atttypmod),
		       NOT a.attnotnull,
		       a.attnum,
		       COALESCE(col_description(a.attrelid, a.attnum), '')
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("listing columns of %q.%q: %w", schemaName, tableName, err)
	}
	defer rows.Close()

	var cols []*model.Column
	for rows.Next() {
		var c model.Column
		if err := rows.Scan(&c.Name, &c.DataType, &c.Nullable, &c.Ordinal, &c.Description); err != nil {
			return nil, err
		}
		cols = append(cols, &c)
	}
	return cols, rows.Err()
}

func (s *Source) primaryKeys(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
		WHERE i.indisprimary AND n.nspname = $1 AND c.relname = $2
		ORDER BY array_position(i.indkey, a.attnum)`, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("listing primary key of %q.%q: %w", schemaName, tableName, err)
	}
	defer rows.Close()
	var pks []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		pks = append(pks, name)
	}
	return pks, rows.Err()
}

// open returns a database/sql handle to the named database on this instance.
func (s *Source) open(dbName string) (*sql.DB, error) {
	return sql.Open("pgx", s.dsn(dbName))
}

// dsn builds a key/value DSN. The password and DSN must never be logged.
func (s *Source) dsn(dbName string) string {
	host, port := splitHostPort(s.conn.HostPort, "5432")
	sslMode := orDefault(s.conn.SSLMode, "prefer")

	parts := []string{
		"host=" + host,
		"port=" + port,
		"user=" + s.conn.Username,
		"dbname=" + dbName,
		"sslmode=" + sslMode,
	}
	if pw := s.conn.Password(); pw != "" {
		parts = append(parts, "password="+pw)
	}
	return strings.Join(parts, " ")
}

func splitHostPort(hostPort, defaultPort string) (string, string) {
	if hostPort == "" {
		return "localhost", defaultPort
	}
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort, defaultPort
	}
	if port == "" {
		port = defaultPort
	}
	return host, port
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
