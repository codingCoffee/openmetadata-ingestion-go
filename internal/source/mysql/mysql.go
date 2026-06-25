// Package mysql extracts schema metadata from a MySQL service.
//
// MySQL has a single namespace level ("schema", synonymous with "database"),
// whereas OpenMetadata models a Database that contains DatabaseSchemas. We map each
// MySQL schema to an OpenMetadata DatabaseSchema under a single logical Database
// named "default" (the OpenMetadata convention used by the Python MySQL connector).
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net"

	"github.com/go-sql-driver/mysql"

	"github.com/zerodha/openmetadata-ingestion-go/internal/config"
	"github.com/zerodha/openmetadata-ingestion-go/internal/filter"
	"github.com/zerodha/openmetadata-ingestion-go/internal/model"
	"github.com/zerodha/openmetadata-ingestion-go/internal/source"
)

// defaultDatabase is the synthetic OpenMetadata Database under which MySQL schemas
// are grouped, matching the Python connector's behaviour.
const defaultDatabase = "default"

func init() {
	source.Register("mysql", New)
}

// Source extracts metadata from a MySQL instance.
type Source struct {
	conn         *config.MySQLConnection
	serviceName  string
	schemaFilter *filter.Filter
	tableFilter  *filter.Filter
	includeViews bool
	includeTabls bool
	db           *sql.DB
}

// New builds a MySQL Source.
func New(src config.Source) (source.Source, error) {
	conn, ok := src.ServiceConnection.Config.(*config.MySQLConnection)
	if !ok {
		return nil, fmt.Errorf("mysql source: unexpected connection type %T", src.ServiceConnection.Config)
	}
	sc := src.SourceConfig.Config
	sf, err := filter.New(filter.Pattern{Includes: sc.SchemaFilterPattern.Includes, Excludes: sc.SchemaFilterPattern.Excludes})
	if err != nil {
		return nil, fmt.Errorf("mysql source: schema filter: %w", err)
	}
	tf, err := filter.New(filter.Pattern{Includes: sc.TableFilterPattern.Includes, Excludes: sc.TableFilterPattern.Excludes})
	if err != nil {
		return nil, fmt.Errorf("mysql source: table filter: %w", err)
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

// Prepare opens the connection pool and pings it.
func (s *Source) Prepare(ctx context.Context) error {
	db, err := sql.Open("mysql", s.dsn())
	if err != nil {
		return err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return err
	}
	s.db = db
	return nil
}

// Close releases the connection pool.
func (s *Source) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Extract pulls schemas, tables and columns into a single logical database.
func (s *Source) Extract(ctx context.Context) (*model.Service, error) {
	svc := &model.Service{Name: s.serviceName, ServiceType: "Mysql"}
	db := &model.Database{Name: defaultDatabase}

	schemaNames, err := s.schemaNames(ctx)
	if err != nil {
		return nil, err
	}
	for _, name := range schemaNames {
		schema := &model.Schema{Name: name}
		tables, err := s.extractTables(ctx, name)
		if err != nil {
			return nil, err
		}
		schema.Tables = tables
		db.Schemas = append(db.Schemas, schema)
	}
	svc.Databases = []*model.Database{db}
	return svc, nil
}

func (s *Source) schemaNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT IN ('mysql','information_schema','performance_schema','sys')
		ORDER BY schema_name`)
	if err != nil {
		return nil, fmt.Errorf("listing schemas: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if !s.schemaFilter.Allowed(name) {
			continue
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *Source) extractTables(ctx context.Context, schemaName string) ([]*model.Table, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT table_name, table_type, COALESCE(table_comment, '')
		FROM information_schema.tables
		WHERE table_schema = ?
		ORDER BY table_name`, schemaName)
	if err != nil {
		return nil, fmt.Errorf("listing tables in %q: %w", schemaName, err)
	}
	defer rows.Close()

	type meta struct {
		name   string
		isView bool
		desc   string
	}
	var metas []meta
	for rows.Next() {
		var name, tableType, comment string
		if err := rows.Scan(&name, &tableType, &comment); err != nil {
			return nil, err
		}
		isView := tableType == "VIEW"
		if isView && !s.includeViews {
			continue
		}
		if !isView && !s.includeTabls {
			continue
		}
		if !s.tableFilter.Allowed(name) {
			continue
		}
		metas = append(metas, meta{name: name, isView: isView, desc: comment})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var tables []*model.Table
	for _, m := range metas {
		cols, err := s.extractColumns(ctx, schemaName, m.name)
		if err != nil {
			return nil, err
		}
		pks, err := s.primaryKeys(ctx, schemaName, m.name)
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

func (s *Source) extractColumns(ctx context.Context, schemaName, tableName string) ([]*model.Column, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT column_name, column_type, is_nullable, ordinal_position, COALESCE(column_comment, '')
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position`, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("listing columns of %q.%q: %w", schemaName, tableName, err)
	}
	defer rows.Close()

	var cols []*model.Column
	for rows.Next() {
		var c model.Column
		var nullable string
		if err := rows.Scan(&c.Name, &c.DataType, &nullable, &c.Ordinal, &c.Description); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		cols = append(cols, &c)
	}
	return cols, rows.Err()
}

func (s *Source) primaryKeys(ctx context.Context, schemaName, tableName string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.statistics
		WHERE table_schema = ? AND table_name = ? AND index_name = 'PRIMARY'
		ORDER BY seq_in_index`, schemaName, tableName)
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

// dsn builds a go-sql-driver/mysql DSN. The password and DSN must never be logged.
func (s *Source) dsn() string {
	host, port := splitHostPort(s.conn.HostPort, "3306")
	cfg := mysql.NewConfig()
	cfg.User = s.conn.Username
	cfg.Passwd = s.conn.Password()
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.DBName = s.conn.Database
	return cfg.FormatDSN()
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
