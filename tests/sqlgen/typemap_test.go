package sqlgen_test

import (
	"strings"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

func TestNativeColumnType(t *testing.T) {
	cases := []struct {
		dialect  string
		abstract string
		length   int
		prec     int
		scale    int
		want     string
	}{
		{"postgresql", "string", 100, 0, 0, "VARCHAR(100)"},
		{"mysql", "string", 100, 0, 0, "VARCHAR(100)"},
		{"sqlite", "string", 100, 0, 0, "VARCHAR(100)"},
		{"postgresql", "string", 0, 0, 0, "VARCHAR(255)"},

		{"postgresql", "text", 0, 0, 0, "TEXT"},
		{"mysql", "text", 0, 0, 0, "TEXT"},
		{"sqlite", "text", 0, 0, 0, "TEXT"},

		{"postgresql", "integer", 0, 0, 0, "INTEGER"},
		{"mysql", "integer", 0, 0, 0, "INT"},
		{"sqlite", "integer", 0, 0, 0, "INTEGER"},

		{"postgresql", "bigint", 0, 0, 0, "BIGINT"},
		{"mysql", "bigint", 0, 0, 0, "BIGINT"},
		{"sqlite", "bigint", 0, 0, 0, "INTEGER"},

		{"postgresql", "boolean", 0, 0, 0, "BOOLEAN"},
		{"mysql", "boolean", 0, 0, 0, "TINYINT(1)"},
		{"sqlite", "boolean", 0, 0, 0, "BOOLEAN"},

		{"postgresql", "uuid", 0, 0, 0, "UUID"},
		{"mysql", "uuid", 0, 0, 0, "CHAR(36)"},
		{"sqlite", "uuid", 0, 0, 0, "TEXT"},

		{"postgresql", "timestamp", 0, 0, 0, "TIMESTAMP"},
		{"mysql", "timestamp", 0, 0, 0, "DATETIME"},
		{"sqlite", "timestamp", 0, 0, 0, "DATETIME"},

		{"postgresql", "date", 0, 0, 0, "DATE"},
		{"mysql", "date", 0, 0, 0, "DATE"},
		{"sqlite", "date", 0, 0, 0, "DATE"},

		{"postgresql", "decimal", 0, 10, 2, "NUMERIC(10,2)"},
		{"mysql", "decimal", 0, 10, 2, "DECIMAL(10,2)"},
		{"sqlite", "decimal", 0, 10, 2, "NUMERIC(10,2)"},
		{"postgresql", "decimal", 0, 0, 0, "NUMERIC(10,0)"},

		{"postgresql", "json", 0, 0, 0, "JSONB"},
		{"mysql", "json", 0, 0, 0, "JSON"},
		{"sqlite", "json", 0, 0, 0, "TEXT"},

		{"postgresql", "float", 0, 0, 0, "DOUBLE PRECISION"},
		{"mysql", "float", 0, 0, 0, "DOUBLE"},
		{"sqlite", "float", 0, 0, 0, "REAL"},
	}

	for _, c := range cases {
		got, err := sqlgen.NativeColumnType(c.dialect, c.abstract, c.length, c.prec, c.scale)
		if err != nil {
			t.Errorf("NativeColumnType(%q, %q): unexpected error: %v", c.dialect, c.abstract, err)
			continue
		}
		if got != c.want {
			t.Errorf("NativeColumnType(%q, %q) = %q, want %q", c.dialect, c.abstract, got, c.want)
		}
	}
}

func TestNativeColumnType_UnknownType(t *testing.T) {
	if _, err := sqlgen.NativeColumnType("postgresql", "money", 0, 0, 0); err == nil {
		t.Error("expected error for unknown abstract type, got nil")
	} else if !strings.Contains(err.Error(), "money") {
		t.Errorf("expected error to mention the unknown type, got: %v", err)
	}
}

func TestNativeColumnType_UnknownDialect(t *testing.T) {
	if _, err := sqlgen.NativeColumnType("oracle", "integer", 0, 0, 0); err == nil {
		t.Error("expected error for unknown dialect, got nil")
	}
}
