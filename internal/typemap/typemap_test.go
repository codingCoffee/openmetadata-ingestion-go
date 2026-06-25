package typemap

import (
	"testing"

	"github.com/open-metadata/openmetadata-sdk/openmetadata-go-client/pkg/ometa"
)

func deref(p *int32) int32 {
	if p == nil {
		return -1
	}
	return *p
}

func TestMapPostgres(t *testing.T) {
	tests := []struct {
		native string
		want   ometa.ColumnDataType
		length int32
		prec   int32
		scale  int32
	}{
		{"integer", ometa.ColumnDataTypeINT, -1, -1, -1},
		{"bigint", ometa.ColumnDataTypeBIGINT, -1, -1, -1},
		{"character varying(255)", ometa.ColumnDataTypeVARCHAR, 255, -1, -1},
		{"numeric(10,2)", ometa.ColumnDataTypeNUMERIC, -1, 10, 2},
		{"timestamp without time zone", ometa.ColumnDataTypeTIMESTAMP, -1, -1, -1},
		{"timestamp with time zone", ometa.ColumnDataTypeTIMESTAMPZ, -1, -1, -1},
		{"boolean", ometa.ColumnDataTypeBOOLEAN, -1, -1, -1},
		{"jsonb", ometa.ColumnDataTypeJSON, -1, -1, -1},
		{"integer[]", ometa.ColumnDataTypeARRAY, -1, -1, -1},
		{"uuid", ometa.ColumnDataTypeUUID, -1, -1, -1},
		{"some_unknown_type", ometa.ColumnDataTypeUNKNOWN, -1, -1, -1},
	}
	for _, tc := range tests {
		t.Run(tc.native, func(t *testing.T) {
			got := Map("Postgres", tc.native)
			if got.DataType != tc.want {
				t.Errorf("DataType = %q, want %q", got.DataType, tc.want)
			}
			if got.Display != tc.native {
				t.Errorf("Display = %q, want %q", got.Display, tc.native)
			}
			if deref(got.Length) != tc.length || deref(got.Precision) != tc.prec || deref(got.Scale) != tc.scale {
				t.Errorf("len/prec/scale = %d/%d/%d, want %d/%d/%d",
					deref(got.Length), deref(got.Precision), deref(got.Scale), tc.length, tc.prec, tc.scale)
			}
		})
	}
}

func TestMapMySQL(t *testing.T) {
	tests := []struct {
		native string
		want   ometa.ColumnDataType
		length int32
		prec   int32
		scale  int32
	}{
		{"int(11)", ometa.ColumnDataTypeINT, -1, -1, -1},
		{"int(10) unsigned", ometa.ColumnDataTypeINT, -1, -1, -1},
		{"varchar(64)", ometa.ColumnDataTypeVARCHAR, 64, -1, -1},
		{"decimal(18,4)", ometa.ColumnDataTypeDECIMAL, -1, 18, 4},
		{"tinyint(1)", ometa.ColumnDataTypeTINYINT, -1, -1, -1},
		{"datetime", ometa.ColumnDataTypeDATETIME, -1, -1, -1},
		{"enum('a','b')", ometa.ColumnDataTypeENUM, -1, -1, -1},
		{"longblob", ometa.ColumnDataTypeLONGBLOB, -1, -1, -1},
		{"json", ometa.ColumnDataTypeJSON, -1, -1, -1},
	}
	for _, tc := range tests {
		t.Run(tc.native, func(t *testing.T) {
			got := Map("Mysql", tc.native)
			if got.DataType != tc.want {
				t.Errorf("DataType = %q, want %q", got.DataType, tc.want)
			}
			if deref(got.Length) != tc.length || deref(got.Precision) != tc.prec || deref(got.Scale) != tc.scale {
				t.Errorf("len/prec/scale = %d/%d/%d, want %d/%d/%d",
					deref(got.Length), deref(got.Precision), deref(got.Scale), tc.length, tc.prec, tc.scale)
			}
		})
	}
}

func TestMapClickHouse(t *testing.T) {
	tests := []struct {
		native string
		want   ometa.ColumnDataType
		prec   int32
		scale  int32
	}{
		{"String", ometa.ColumnDataTypeSTRING, -1, -1},
		{"UInt64", ometa.ColumnDataTypeBIGINT, -1, -1},
		{"Int32", ometa.ColumnDataTypeINT, -1, -1},
		{"Float64", ometa.ColumnDataTypeDOUBLE, -1, -1},
		{"Nullable(String)", ometa.ColumnDataTypeSTRING, -1, -1},
		{"LowCardinality(String)", ometa.ColumnDataTypeSTRING, -1, -1},
		{"Nullable(Decimal(18, 2))", ometa.ColumnDataTypeDECIMAL, 18, 2},
		{"Array(String)", ometa.ColumnDataTypeARRAY, -1, -1},
		{"DateTime64(3)", ometa.ColumnDataTypeDATETIME, -1, -1},
		{"Enum8('a' = 1)", ometa.ColumnDataTypeENUM, -1, -1},
	}
	for _, tc := range tests {
		t.Run(tc.native, func(t *testing.T) {
			got := Map("Clickhouse", tc.native)
			if got.DataType != tc.want {
				t.Errorf("DataType = %q, want %q", got.DataType, tc.want)
			}
			if deref(got.Precision) != tc.prec || deref(got.Scale) != tc.scale {
				t.Errorf("prec/scale = %d/%d, want %d/%d",
					deref(got.Precision), deref(got.Scale), tc.prec, tc.scale)
			}
		})
	}
}
