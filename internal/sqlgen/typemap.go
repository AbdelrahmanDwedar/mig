package sqlgen

import "fmt"

// nativeTypeTemplates holds the per-dialect SQL type template for each
// portable abstract type. Templates containing "%d" are filled in with
// length/precision/scale by nativeColumnType.
type nativeTypeTemplates struct {
	Postgres string
	MySQL    string
	SQLite   string
}

var typeMap = map[string]nativeTypeTemplates{
	"string":    {"VARCHAR(%d)", "VARCHAR(%d)", "VARCHAR(%d)"},
	"text":      {"TEXT", "TEXT", "TEXT"},
	"integer":   {"INTEGER", "INT", "INTEGER"},
	"bigint":    {"BIGINT", "BIGINT", "INTEGER"},
	"boolean":   {"BOOLEAN", "TINYINT(1)", "BOOLEAN"},
	"uuid":      {"UUID", "CHAR(36)", "TEXT"},
	"timestamp": {"TIMESTAMP", "DATETIME", "DATETIME"},
	"date":      {"DATE", "DATE", "DATE"},
	"decimal":   {"NUMERIC(%d,%d)", "DECIMAL(%d,%d)", "NUMERIC(%d,%d)"},
	"json":      {"JSONB", "JSON", "TEXT"},
	"float":     {"DOUBLE PRECISION", "DOUBLE", "REAL"},
}

const (
	defaultStringLength = 255
	defaultDecimalPrec  = 10
	defaultDecimalScale = 0
)

// NativeColumnType resolves an abstract column type to its native SQL
// rendering for the given dialect name ("postgresql", "mysql", "sqlite").
func NativeColumnType(dialectName, abstractType string, length, precision, scale int) (string, error) {
	tmpl, ok := typeMap[abstractType]
	if !ok {
		return "", fmt.Errorf("unsupported column type: %q", abstractType)
	}

	var t string
	switch dialectName {
	case "postgresql":
		t = tmpl.Postgres
	case "mysql":
		t = tmpl.MySQL
	case "sqlite":
		t = tmpl.SQLite
	default:
		return "", fmt.Errorf("unsupported dialect: %q", dialectName)
	}

	switch abstractType {
	case "string":
		if length == 0 {
			length = defaultStringLength
		}
		return fmt.Sprintf(t, length), nil
	case "decimal":
		if precision == 0 {
			precision = defaultDecimalPrec
		}
		if scale == 0 {
			scale = defaultDecimalScale
		}
		return fmt.Sprintf(t, precision, scale), nil
	default:
		return t, nil
	}
}
