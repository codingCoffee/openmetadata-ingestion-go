// Package typemap normalises native database column type strings into the
// OpenMetadata ColumnDataType enum, preserving the original string for display and
// extracting length/precision/scale where present. There is one mapping table per
// supported source; unmapped types fall back to UNKNOWN.
package typemap

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/open-metadata/openmetadata-sdk/openmetadata-go-client/pkg/ometa"
)

// Result is the outcome of mapping a native type.
type Result struct {
	DataType  ometa.ColumnDataType
	Display   string // the original native type string
	Length    *int32 // character length, where applicable
	Precision *int32 // numeric precision, where applicable
	Scale     *int32 // numeric scale, where applicable
}

var parenNums = regexp.MustCompile(`\(\s*([0-9]+)\s*(?:,\s*([0-9]+)\s*)?\)`)

// Map normalises native into an OpenMetadata data type for the given service type
// ("Postgres", "Mysql", "Clickhouse"). It never fails: unknown types map to UNKNOWN.
func Map(serviceType, native string) Result {
	res := Result{Display: strings.TrimSpace(native), DataType: ometa.ColumnDataTypeUNKNOWN}
	switch strings.ToLower(serviceType) {
	case "postgres", "postgresql":
		mapPostgres(res.Display, &res)
	case "mysql":
		mapMySQL(res.Display, &res)
	case "clickhouse":
		mapClickHouse(res.Display, &res)
	}
	return res
}

// numericParams parses up to two numbers from the first "(p[,s])" group in s.
func numericParams(s string) (p, q *int32, ok bool) {
	m := parenNums.FindStringSubmatch(s)
	if m == nil {
		return nil, nil, false
	}
	if m[1] != "" {
		p = parseI32(m[1])
	}
	if m[2] != "" {
		q = parseI32(m[2])
	}
	return p, q, true
}

func parseI32(s string) *int32 {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return nil
	}
	v := int32(n)
	return &v
}

// applyParams assigns parsed parens to length or precision/scale depending on dt.
func applyParams(native string, dt ometa.ColumnDataType, res *Result) {
	a, b, ok := numericParams(native)
	if !ok {
		return
	}
	switch dt {
	case ometa.ColumnDataTypeDECIMAL, ometa.ColumnDataTypeNUMERIC, ometa.ColumnDataTypeNUMBER:
		res.Precision, res.Scale = a, b
	case ometa.ColumnDataTypeVARCHAR, ometa.ColumnDataTypeCHAR, ometa.ColumnDataTypeBINARY,
		ometa.ColumnDataTypeVARBINARY, ometa.ColumnDataTypeSTRING:
		if b == nil {
			res.Length = a
		}
	}
}

func mapPostgres(native string, res *Result) {
	lower := strings.ToLower(strings.TrimSpace(native))
	if strings.HasSuffix(lower, "[]") {
		res.DataType = ometa.ColumnDataTypeARRAY
		return
	}
	// Drop parenthesised modifiers ("(255)", "(10,2)") to recover the type name,
	// which for Postgres may be multi-word ("character varying", "timestamp without time zone").
	name := strings.TrimSpace(parenNums.ReplaceAllString(lower, ""))
	name = strings.Join(strings.Fields(name), " ")
	if dt, ok := postgresTypes[name]; ok {
		res.DataType = dt
		applyParams(lower, dt, res)
	}
}

func mapMySQL(native string, res *Result) {
	lower := strings.ToLower(strings.TrimSpace(native))
	lower = strings.ReplaceAll(lower, " unsigned", "")
	lower = strings.ReplaceAll(lower, " zerofill", "")
	lower = strings.ReplaceAll(lower, " signed", "")
	base := baseToken(lower)
	if dt, ok := mysqlTypes[base]; ok {
		res.DataType = dt
		// enum/set parentheses carry value lists, not lengths.
		if base != "enum" && base != "set" {
			applyParams(lower, dt, res)
		}
	}
}

func mapClickHouse(native string, res *Result) {
	t := strings.TrimSpace(native)
	// Unwrap modifier wrappers, keeping the inner type. Nullability is recorded by
	// the source, not here.
	for {
		inner, ok := unwrap(t, "Nullable")
		if !ok {
			inner, ok = unwrap(t, "LowCardinality")
		}
		if !ok {
			break
		}
		t = inner
	}
	lower := strings.ToLower(t)
	base := baseToken(lower)
	if dt, ok := clickhouseTypes[base]; ok {
		res.DataType = dt
		applyParams(lower, dt, res)
	}
}

// unwrap strips a "Wrapper(...)" enclosing t, returning the inner content.
func unwrap(t, wrapper string) (string, bool) {
	prefix := wrapper + "("
	if strings.HasPrefix(t, prefix) && strings.HasSuffix(t, ")") {
		return strings.TrimSpace(t[len(prefix) : len(t)-1]), true
	}
	return "", false
}

// baseToken returns the type name before any "(".
func baseToken(s string) string {
	if i := strings.IndexByte(s, '('); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

var postgresTypes = map[string]ometa.ColumnDataType{
	"smallint":                    ometa.ColumnDataTypeSMALLINT,
	"int2":                        ometa.ColumnDataTypeSMALLINT,
	"integer":                     ometa.ColumnDataTypeINT,
	"int":                         ometa.ColumnDataTypeINT,
	"int4":                        ometa.ColumnDataTypeINT,
	"bigint":                      ometa.ColumnDataTypeBIGINT,
	"int8":                        ometa.ColumnDataTypeBIGINT,
	"boolean":                     ometa.ColumnDataTypeBOOLEAN,
	"bool":                        ometa.ColumnDataTypeBOOLEAN,
	"real":                        ometa.ColumnDataTypeFLOAT,
	"float4":                      ometa.ColumnDataTypeFLOAT,
	"double precision":            ometa.ColumnDataTypeDOUBLE,
	"float8":                      ometa.ColumnDataTypeDOUBLE,
	"numeric":                     ometa.ColumnDataTypeNUMERIC,
	"decimal":                     ometa.ColumnDataTypeDECIMAL,
	"money":                       ometa.ColumnDataTypeMONEY,
	"character varying":           ometa.ColumnDataTypeVARCHAR,
	"varchar":                     ometa.ColumnDataTypeVARCHAR,
	"character":                   ometa.ColumnDataTypeCHAR,
	"char":                        ometa.ColumnDataTypeCHAR,
	"bpchar":                      ometa.ColumnDataTypeCHAR,
	"text":                        ometa.ColumnDataTypeTEXT,
	"date":                        ometa.ColumnDataTypeDATE,
	"timestamp":                   ometa.ColumnDataTypeTIMESTAMP,
	"timestamp without time zone": ometa.ColumnDataTypeTIMESTAMP,
	"timestamp with time zone":    ometa.ColumnDataTypeTIMESTAMPZ,
	"time":                        ometa.ColumnDataTypeTIME,
	"time without time zone":      ometa.ColumnDataTypeTIME,
	"time with time zone":         ometa.ColumnDataTypeTIME,
	"interval":                    ometa.ColumnDataTypeINTERVAL,
	"json":                        ometa.ColumnDataTypeJSON,
	"jsonb":                       ometa.ColumnDataTypeJSON,
	"uuid":                        ometa.ColumnDataTypeUUID,
	"bytea":                       ometa.ColumnDataTypeBYTEA,
	"inet":                        ometa.ColumnDataTypeINET,
	"cidr":                        ometa.ColumnDataTypeCIDR,
	"macaddr":                     ometa.ColumnDataTypeMACADDR,
	"bit":                         ometa.ColumnDataTypeBIT,
	"xml":                         ometa.ColumnDataTypeXML,
	"point":                       ometa.ColumnDataTypePOINT,
	"polygon":                     ometa.ColumnDataTypePOLYGON,
	"tsvector":                    ometa.ColumnDataTypeTSVECTOR,
	"tsquery":                     ometa.ColumnDataTypeTSQUERY,
}

var mysqlTypes = map[string]ometa.ColumnDataType{
	"tinyint":    ometa.ColumnDataTypeTINYINT,
	"smallint":   ometa.ColumnDataTypeSMALLINT,
	"mediumint":  ometa.ColumnDataTypeINT,
	"int":        ometa.ColumnDataTypeINT,
	"integer":    ometa.ColumnDataTypeINT,
	"bigint":     ometa.ColumnDataTypeBIGINT,
	"bit":        ometa.ColumnDataTypeBIT,
	"boolean":    ometa.ColumnDataTypeBOOLEAN,
	"bool":       ometa.ColumnDataTypeBOOLEAN,
	"decimal":    ometa.ColumnDataTypeDECIMAL,
	"dec":        ometa.ColumnDataTypeDECIMAL,
	"numeric":    ometa.ColumnDataTypeNUMERIC,
	"fixed":      ometa.ColumnDataTypeDECIMAL,
	"float":      ometa.ColumnDataTypeFLOAT,
	"double":     ometa.ColumnDataTypeDOUBLE,
	"char":       ometa.ColumnDataTypeCHAR,
	"varchar":    ometa.ColumnDataTypeVARCHAR,
	"tinytext":   ometa.ColumnDataTypeTEXT,
	"text":       ometa.ColumnDataTypeTEXT,
	"mediumtext": ometa.ColumnDataTypeMEDIUMTEXT,
	"longtext":   ometa.ColumnDataTypeTEXT,
	"binary":     ometa.ColumnDataTypeBINARY,
	"varbinary":  ometa.ColumnDataTypeVARBINARY,
	"tinyblob":   ometa.ColumnDataTypeBLOB,
	"blob":       ometa.ColumnDataTypeBLOB,
	"mediumblob": ometa.ColumnDataTypeMEDIUMBLOB,
	"longblob":   ometa.ColumnDataTypeLONGBLOB,
	"date":       ometa.ColumnDataTypeDATE,
	"datetime":   ometa.ColumnDataTypeDATETIME,
	"timestamp":  ometa.ColumnDataTypeTIMESTAMP,
	"time":       ometa.ColumnDataTypeTIME,
	"year":       ometa.ColumnDataTypeYEAR,
	"json":       ometa.ColumnDataTypeJSON,
	"enum":       ometa.ColumnDataTypeENUM,
	"set":        ometa.ColumnDataTypeSET,
	"geometry":   ometa.ColumnDataTypeGEOMETRY,
	"point":      ometa.ColumnDataTypePOINT,
}

var clickhouseTypes = map[string]ometa.ColumnDataType{
	"int8":        ometa.ColumnDataTypeTINYINT,
	"uint8":       ometa.ColumnDataTypeTINYINT,
	"int16":       ometa.ColumnDataTypeSMALLINT,
	"uint16":      ometa.ColumnDataTypeSMALLINT,
	"int32":       ometa.ColumnDataTypeINT,
	"uint32":      ometa.ColumnDataTypeINT,
	"int64":       ometa.ColumnDataTypeBIGINT,
	"uint64":      ometa.ColumnDataTypeBIGINT,
	"int128":      ometa.ColumnDataTypeBIGINT,
	"uint128":     ometa.ColumnDataTypeBIGINT,
	"int256":      ometa.ColumnDataTypeBIGINT,
	"uint256":     ometa.ColumnDataTypeBIGINT,
	"float32":     ometa.ColumnDataTypeFLOAT,
	"float64":     ometa.ColumnDataTypeDOUBLE,
	"decimal":     ometa.ColumnDataTypeDECIMAL,
	"decimal32":   ometa.ColumnDataTypeDECIMAL,
	"decimal64":   ometa.ColumnDataTypeDECIMAL,
	"decimal128":  ometa.ColumnDataTypeDECIMAL,
	"decimal256":  ometa.ColumnDataTypeDECIMAL,
	"bool":        ometa.ColumnDataTypeBOOLEAN,
	"boolean":     ometa.ColumnDataTypeBOOLEAN,
	"string":      ometa.ColumnDataTypeSTRING,
	"fixedstring": ometa.ColumnDataTypeSTRING,
	"uuid":        ometa.ColumnDataTypeUUID,
	"date":        ometa.ColumnDataTypeDATE,
	"date32":      ometa.ColumnDataTypeDATE,
	"datetime":    ometa.ColumnDataTypeDATETIME,
	"datetime64":  ometa.ColumnDataTypeDATETIME,
	"array":       ometa.ColumnDataTypeARRAY,
	"tuple":       ometa.ColumnDataTypeTUPLE,
	"map":         ometa.ColumnDataTypeMAP,
	"enum8":       ometa.ColumnDataTypeENUM,
	"enum16":      ometa.ColumnDataTypeENUM,
	"ipv4":        ometa.ColumnDataTypeIPV4,
	"ipv6":        ometa.ColumnDataTypeIPV6,
	"json":        ometa.ColumnDataTypeJSON,
}
