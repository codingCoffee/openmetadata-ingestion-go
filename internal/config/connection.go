package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConnectionConfig is implemented by every concrete source connection type. The
// concrete type is chosen from the YAML config.type discriminator.
type ConnectionConfig interface {
	// ConnType returns the OpenMetadata connection type string (e.g. "Postgres").
	ConnType() string
	// Base returns the shared connection fields.
	Base() BaseConnection
}

// ServiceConnection wraps the polymorphic serviceConnection.config block.
type ServiceConnection struct {
	Config ConnectionConfig
}

// AuthType mirrors the Python authType block. Only password auth is supported.
type AuthType struct {
	Password string `yaml:"password"`
}

// BaseConnection holds the connection fields common to all supported sources.
type BaseConnection struct {
	Type               string   `yaml:"type"`
	Username           string   `yaml:"username"`
	AuthType           AuthType `yaml:"authType"`
	HostPort           string   `yaml:"hostPort"`
	Database           string   `yaml:"database"`
	IngestAllDatabases bool     `yaml:"ingestAllDatabases"`
}

func (b BaseConnection) Password() string {
	return b.AuthType.Password
}

// PostgresConnection is the PostgreSQL serviceConnection.config.
type PostgresConnection struct {
	BaseConnection `yaml:",inline"`
	SSLMode        string `yaml:"sslMode"`
}

func (c *PostgresConnection) ConnType() string     { return "Postgres" }
func (c *PostgresConnection) Base() BaseConnection { return c.BaseConnection }

// MySQLConnection is the MySQL serviceConnection.config.
type MySQLConnection struct {
	BaseConnection `yaml:",inline"`
	DatabaseSchema string `yaml:"databaseSchema"`
}

func (c *MySQLConnection) ConnType() string     { return "Mysql" }
func (c *MySQLConnection) Base() BaseConnection { return c.BaseConnection }

// ClickHouseConnection is the ClickHouse serviceConnection.config.
type ClickHouseConnection struct {
	BaseConnection `yaml:",inline"`
	Secure         bool `yaml:"secure"`
	HTTPS          bool `yaml:"https"`
}

func (c *ClickHouseConnection) ConnType() string     { return "Clickhouse" }
func (c *ClickHouseConnection) Base() BaseConnection { return c.BaseConnection }

// UnmarshalYAML performs a two-pass decode: pass one reads config.type to pick the
// concrete connection struct, pass two decodes the config block into it.
func (s *ServiceConnection) UnmarshalYAML(value *yaml.Node) error {
	var probe struct {
		Config struct {
			Type string `yaml:"type"`
		} `yaml:"config"`
	}
	if err := value.Decode(&probe); err != nil {
		return err
	}

	switch strings.ToLower(probe.Config.Type) {
	case "postgres", "postgresql":
		s.Config = &PostgresConnection{}
	case "mysql":
		s.Config = &MySQLConnection{}
	case "clickhouse":
		s.Config = &ClickHouseConnection{}
	default:
		return fmt.Errorf("unsupported serviceConnection.config.type %q (supported: PostgreSQL, Mysql, Clickhouse)", probe.Config.Type)
	}

	var raw struct {
		Config yaml.Node `yaml:"config"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	return raw.Config.Decode(s.Config)
}
