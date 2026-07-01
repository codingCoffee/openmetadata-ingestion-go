package sink

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-metadata/openmetadata-sdk/openmetadata-go-client/pkg/ometa"

	"github.com/codingcoffee/openmetadata-ingestion-go/internal/config"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/fqn"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/model"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/typemap"
)

// serviceTypes maps our model service type to the SDK's CreateDatabaseService enum.
var serviceTypes = map[string]ometa.CreateDatabaseServiceServiceType{
	"Postgres":   ometa.CreateDatabaseServiceServiceTypePostgres,
	"Mysql":      ometa.CreateDatabaseServiceServiceTypeMysql,
	"Clickhouse": ometa.CreateDatabaseServiceServiceTypeClickhouse,
}

// MetadataREST pushes entities to OpenMetadata via the official SDK. Every write
// uses CreateOrUpdate so re-running an ingestion is idempotent.
type MetadataREST struct {
	client *ometa.Client
	// serviceType is captured on UpsertService and used to select the per-source
	// type mapping when building table columns.
	serviceType string
}

// NewMetadataREST builds a sink from the OpenMetadata server config. The JWT token
// must never be logged.
func NewMetadataREST(cfg config.OpenMetadataServerConfig) (Sink, error) {
	if cfg.HostPort == "" {
		return nil, fmt.Errorf("openMetadataServerConfig.hostPort is required")
	}
	// The Python ingestion config format (which we mirror) sets hostPort with an
	// "/api" suffix, e.g. "http://host:8585/api". The SDK backend appends "/api/v1"
	// itself, so passing the suffixed value through verbatim doubles it into
	// ".../api/api/v1/..." and the server returns 404/405. Strip a trailing "/api"
	// (and surrounding slashes) so both forms of hostPort resolve correctly.
	baseURL := strings.TrimRight(cfg.HostPort, "/")
	baseURL = strings.TrimSuffix(baseURL, "/api")
	baseURL = strings.TrimRight(baseURL, "/")
	client := ometa.NewClient(baseURL, ometa.WithToken(cfg.SecurityConfig.JWTToken))
	return &MetadataREST{client: client}, nil
}

// Close is a no-op; the SDK client holds no long-lived resources to release.
func (m *MetadataREST) Close() error { return nil }

func (m *MetadataREST) UpsertService(ctx context.Context, svc *model.Service) (string, error) {
	st, ok := serviceTypes[svc.ServiceType]
	if !ok {
		return "", fmt.Errorf("unsupported service type %q", svc.ServiceType)
	}
	m.serviceType = svc.ServiceType
	_, err := m.client.DatabaseServices.CreateOrUpdate(ctx, &ometa.CreateDatabaseService{
		Name:        svc.Name,
		ServiceType: st,
	})
	if err != nil {
		return "", err
	}
	return fqn.Build(svc.Name), nil
}

func (m *MetadataREST) UpsertDatabase(ctx context.Context, serviceFQN string, db *model.Database) (string, error) {
	_, err := m.client.Databases.CreateOrUpdate(ctx, &ometa.CreateDatabase{
		Name:        db.Name,
		Service:     serviceFQN,
		Description: strPtr(db.Description),
	})
	if err != nil {
		return "", err
	}
	return fqn.Append(serviceFQN, db.Name), nil
}

func (m *MetadataREST) UpsertSchema(ctx context.Context, dbFQN string, schema *model.Schema) (string, error) {
	_, err := m.client.DatabaseSchemas.CreateOrUpdate(ctx, &ometa.CreateDatabaseSchema{
		Name:        schema.Name,
		Database:    dbFQN,
		Description: strPtr(schema.Description),
	})
	if err != nil {
		return "", err
	}
	return fqn.Append(dbFQN, schema.Name), nil
}

func (m *MetadataREST) UpsertTable(ctx context.Context, schemaFQN string, table *model.Table) (string, error) {
	body := &ometa.CreateTable{
		Name:           table.Name,
		DatabaseSchema: schemaFQN,
		Description:    strPtr(table.Description),
		Columns:        m.columns(table),
		TableType:      tableType(table.IsView),
	}
	if len(table.PrimaryKeys) > 0 {
		pkCols := append([]string(nil), table.PrimaryKeys...)
		body.TableConstraints = &[]ometa.TableConstraint{{
			ConstraintType: ometa.Ptr(ometa.TableConstraintConstraintTypePRIMARYKEY),
			Columns:        &pkCols,
		}}
	}
	if _, err := m.client.Tables.CreateOrUpdate(ctx, body); err != nil {
		return "", err
	}
	return fqn.Append(schemaFQN, table.Name), nil
}

func (m *MetadataREST) columns(table *model.Table) []ometa.Column {
	cols := make([]ometa.Column, 0, len(table.Columns))
	for _, c := range table.Columns {
		mapped := typemap.Map(m.serviceType, c.DataType)
		col := ometa.Column{
			Name:            c.Name,
			DataType:        mapped.DataType,
			DataTypeDisplay: strPtr(mapped.Display),
			Description:     strPtr(c.Description),
			DataLength:      mapped.Length,
			Precision:       mapped.Precision,
			Scale:           mapped.Scale,
		}
		if c.Ordinal > 0 {
			col.OrdinalPosition = ometa.Int32(int32(c.Ordinal))
		}
		cols = append(cols, col)
	}
	return cols
}

func tableType(isView bool) *ometa.CreateTableTableType {
	if isView {
		return ometa.Ptr(ometa.CreateTableTableTypeView)
	}
	return ometa.Ptr(ometa.CreateTableTableTypeRegular)
}

// strPtr returns nil for empty strings so optional fields are omitted from the
// request rather than sent as "".
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return ometa.Str(s)
}
