# openmetadata-ingestion-go

A small, single-binary metadata ingestion tool for [OpenMetadata](https://open-metadata.org),
written in Go. It connects to a relational database, extracts its schema metadata, and pushes
it to an OpenMetadata server via the official
[Go SDK](https://pkg.go.dev/github.com/open-metadata/openmetadata-sdk/openmetadata-go-client/pkg/ometa).

It is a focused Go alternative to a subset of the Python
[`openmetadata-ingestion`](https://pypi.org/project/openmetadata-ingestion/) package, intended
for pushing database metadata from servers without a Python runtime. It deliberately reuses the
**same workflow YAML format** as the Python connectors, so existing configs work unchanged.

## Supported sources

| Source     | Connection `type` | Extracted                                            |
|------------|-------------------|------------------------------------------------------|
| PostgreSQL | `PostgreSQL`      | databases, schemas, tables, views, columns, PKs      |
| MySQL      | `Mysql`           | schemas, tables, views, columns, PKs                 |
| ClickHouse | `Clickhouse`      | databases (as schemas), tables, views, columns, PKs  |

For every source the tool also extracts **table/column descriptions** (comments) and applies
**include/exclude regex filtering** for schemas and tables.

## Entities pushed

`DatabaseService` → `Database` → `DatabaseSchema` → `Table` (with `Column`s, primary-key
constraints, and table/view classification). Every write uses `CreateOrUpdate`, so re-running an
ingestion is idempotent.

## Install / build

Requires Go 1.24+.

```bash
go build -o omingest ./cmd/omingest
```

## Usage

```bash
# Push metadata
omingest ingest -c workflow.yaml

# Just check the source DB is reachable
omingest test-connection -c workflow.yaml

# Override the configured log level
omingest ingest -c workflow.yaml --log-level DEBUG
```

### Configuration

The config is compatible with the Python ingestion workflow YAML. See
[`testdata/workflow.postgres.yaml`](testdata/workflow.postgres.yaml) for a complete example:

```yaml
source:
  type: postgres
  serviceName: local_postgres
  serviceConnection:
    config:
      type: PostgreSQL
      username: postgres
      authType:
        password: postgres
      hostPort: localhost:5432
      database: postgres
      ingestAllDatabases: false
      sslMode: disable
  sourceConfig:
    config:
      type: DatabaseMetadata
      includeTables: true
      includeViews: true
      schemaFilterPattern:
        includes: [public]
        excludes: ["^pg_"]
      tableFilterPattern:
        excludes: ["_tmp$"]
sink:
  type: metadata-rest
  config: {}
workflowConfig:
  loggerLevel: INFO
  openMetadataServerConfig:
    hostPort: "http://localhost:8585/api"
    authProvider: openmetadata
    securityConfig:
      jwtToken: "<bot-jwt-token>"
```

Unknown YAML keys (Python options this tool does not implement) are ignored, not rejected.

Filter precedence matches the Python connectors: an exclude match rejects immediately; if there
are no includes, everything else is allowed; otherwise a name must match an include. Patterns are
unanchored regular expressions (`re.search` semantics).

### Secrets

The DB password and the OpenMetadata JWT are read from the config and are never logged. Keep your
workflow YAML out of version control, or template the secret values in from your secret manager
at deploy time.

## Notes / not yet implemented

- `markDeletedTables` is parsed but not yet acted upon (no soft-delete of entities that disappear
  between runs).
- No profiler, sample data, lineage, or query-usage ingestion.
- MySQL and ClickHouse have a single namespace level; each is mapped to an OpenMetadata
  `DatabaseSchema` under a single logical `Database` named `default`.

## Development

```bash
go test ./...      # unit tests (no DB or network required)
go vet ./...
```
