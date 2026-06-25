// Package clickhouse extracts schema metadata from a ClickHouse service.
//
// ClickHouse, like MySQL, has a single namespace level (its "database"). We map each
// ClickHouse database to an OpenMetadata DatabaseSchema under a single logical
// OpenMetadata Database named "default", matching the Python ClickHouse connector.
package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"

	_ "github.com/ClickHouse/clickhouse-go/v2" // register the "clickhouse" database/sql driver

	"github.com/zerodha/openmetadata-ingestion-go/internal/config"
	"github.com/zerodha/openmetadata-ingestion-go/internal/filter"
	"github.com/zerodha/openmetadata-ingestion-go/internal/model"
	"github.com/zerodha/openmetadata-ingestion-go/internal/source"
)

const defaultDatabase = "default"

func init() {
	source.Register("clickhouse", New)
}

// Source extracts metadata from a ClickHouse instance.
type Source struct {
	conn         *config.ClickHouseConnection
	serviceName  string
	schemaFilter *filter.Filter
	tableFilter  *filter.Filter
	includeViews bool
	includeTabls bool
	db           *sql.DB
}

// New builds a ClickHouse Source.
func New(src config.Source) (source.Source, error) {
	conn, ok := src.ServiceConnection.Config.(*config.ClickHouseConnection)
	if !ok {
		return nil, fmt.Errorf("clickhouse source: unexpected connection type %T", src.ServiceConnection.Config)
	}
	sc := src.SourceConfig.Config
	sf, err := filter.New(filter.Pattern{Includes: sc.SchemaFilterPattern.Includes, Excludes: sc.SchemaFilterPattern.Excludes})
	if err != nil {
		return nil, fmt.Errorf("clickhouse source: schema filter: %w", err)
	}
	tf, err := filter.New(filter.Pattern{Includes: sc.TableFilterPattern.Includes, Excludes: sc.TableFilterPattern.Excludes})
	if err != nil {
		return nil, fmt.Errorf("clickhouse source: table filter: %w", err)
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
	db, err := sql.Open("clickhouse", s.dsn())
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

// Extract pulls ClickHouse databases (as schemas), tables and columns.
func (s *Source) Extract(ctx context.Context) (*model.Service, error) {
	svc := &model.Service{Name: s.serviceName, ServiceType: "Clickhouse"}
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
		SELECT name FROM system.databases
		WHERE name NOT IN ('system','information_schema','INFORMATION_SCHEMA')
		ORDER BY name`)
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
		if !s.schemaFilter.Allowed(name) {
			continue
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (s *Source) extractTables(ctx context.Context, schemaName string) ([]*model.Table, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, engine, comment
		FROM system.tables
		WHERE database = ?
		ORDER BY name`, schemaName)
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
		var name, engine, comment string
		if err := rows.Scan(&name, &engine, &comment); err != nil {
			return nil, err
		}
		isView := engine == "View" || engine == "MaterializedView" || engine == "LiveView"
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
		cols, pks, err := s.extractColumns(ctx, schemaName, m.name)
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

// extractColumns returns the columns and the primary-key column names. ClickHouse
// has no primary-key constraint; the sorting/primary key is exposed via the
// is_in_primary_key flag on system.columns.
func (s *Source) extractColumns(ctx context.Context, schemaName, tableName string) ([]*model.Column, []string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, type, comment, position, is_in_primary_key
		FROM system.columns
		WHERE database = ? AND table = ?
		ORDER BY position`, schemaName, tableName)
	if err != nil {
		return nil, nil, fmt.Errorf("listing columns of %q.%q: %w", schemaName, tableName, err)
	}
	defer rows.Close()

	var cols []*model.Column
	var pks []string
	for rows.Next() {
		var c model.Column
		var inPK uint8
		if err := rows.Scan(&c.Name, &c.DataType, &c.Description, &c.Ordinal, &inPK); err != nil {
			return nil, nil, err
		}
		// A column wrapped in Nullable(...) is nullable; the typemap package unwraps
		// the type for OpenMetadata, but nullability is captured here for accuracy.
		c.Nullable = hasNullableWrapper(c.DataType)
		cols = append(cols, &c)
		if inPK == 1 {
			pks = append(pks, c.Name)
		}
	}
	return cols, pks, rows.Err()
}

func hasNullableWrapper(t string) bool {
	return len(t) >= 9 && (t[:9] == "Nullable(" )
}

// dsn builds a clickhouse:// DSN URL. The password and DSN must never be logged.
func (s *Source) dsn() string {
	host, port := splitHostPort(s.conn.HostPort, "9000")
	u := url.URL{
		Scheme: "clickhouse",
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + s.conn.Database,
	}
	if s.conn.Username != "" {
		if pw := s.conn.Password(); pw != "" {
			u.User = url.UserPassword(s.conn.Username, pw)
		} else {
			u.User = url.User(s.conn.Username)
		}
	}
	if s.conn.Secure || s.conn.HTTPS {
		q := u.Query()
		q.Set("secure", "true")
		u.RawQuery = q.Encode()
	}
	return u.String()
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
