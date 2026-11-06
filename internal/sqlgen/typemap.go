package sqlgen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var typeOrder = []string{
	"string", "text", "integer", "bigint", "boolean",
	"uuid", "timestamp", "date", "decimal", "json", "float",
}

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

func IsValidAbstractType(t string) bool {
	_, ok := typeMap[t]
	return ok
}

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

func AbstractType(dialectName, nativeType string) (abstractType string, length, precision, scale int, ok bool) {
	native := strings.TrimSpace(nativeType)

	for _, abstract := range typeOrder {
		tmpl, exists := typeMap[abstract]
		if !exists {
			continue
		}

		var pattern string
		switch dialectName {
		case "postgresql":
			pattern = tmpl.Postgres
		case "mysql":
			pattern = tmpl.MySQL
		case "sqlite":
			pattern = tmpl.SQLite
		default:
			return "", 0, 0, 0, false
		}

		m := templateRegexp(pattern).FindStringSubmatch(native)
		if m == nil {
			continue
		}

		switch abstract {
		case "string":
			length = defaultStringLength
			if len(m) > 1 && m[1] != "" {
				if n, err := strconv.Atoi(m[1]); err == nil {
					length = n
				}
			}
		case "decimal":
			precision, scale = defaultDecimalPrec, defaultDecimalScale
			if len(m) > 2 {
				if n, err := strconv.Atoi(m[1]); err == nil {
					precision = n
				}
				if n, err := strconv.Atoi(m[2]); err == nil {
					scale = n
				}
			}
		}
		return abstract, length, precision, scale, true
	}
	return "", 0, 0, 0, false
}

func templateRegexp(tmpl string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(tmpl)
	escaped = strings.ReplaceAll(escaped, `%d`, `(\d+)`)
	return regexp.MustCompile(`(?i)^` + escaped + `$`)
}
