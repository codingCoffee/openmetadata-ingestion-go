// Package model defines a source-neutral representation of the database metadata
// hierarchy. Sources extract into these types; the sink translates them into the
// OpenMetadata SDK request types. Keeping this layer SDK-free makes extraction,
// filtering and type-mapping unit-testable without the SDK.
package model

// Service is the root of an extraction: a single database service (e.g. one
// Postgres/MySQL/ClickHouse instance) and all of the databases discovered under it.
type Service struct {
	Name        string // serviceName from config
	ServiceType string // OpenMetadata service type, e.g. "Postgres", "Mysql", "Clickhouse"
	Databases   []*Database
}

// Database is a logical database within a service.
type Database struct {
	Name        string
	Description string
	Schemas     []*Schema
}

// Schema is a namespace within a database holding tables and views.
type Schema struct {
	Name        string
	Description string
	Tables      []*Table
}

// Table represents a table or view and its columns.
type Table struct {
	Name        string
	Description string
	IsView      bool
	Columns     []*Column
	// PrimaryKeys lists the column names participating in the primary key,
	// in key order. Empty when the table has no primary key.
	PrimaryKeys []string
}

// Column is a single column of a table.
type Column struct {
	Name        string
	Description string
	// DataType is the native database type string exactly as reported by the
	// source catalog (e.g. "character varying(255)", "Nullable(String)"). The
	// typemap package normalises this into an OpenMetadata column data type while
	// preserving the original for display.
	DataType string
	Nullable bool
	// Ordinal is the 1-based position of the column within the table.
	Ordinal int
}
