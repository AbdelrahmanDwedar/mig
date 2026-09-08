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

// TestAbstractType_RoundTrip checks that every abstract type in typeMap
// reverse-maps back to itself (with length/precision/scale preserved) for
// every dialect, EXCEPT the SQLite collisions documented on typeOrder
// (bigint/integer and uuid/json/text all render as the same native SQLite
// type, so they cannot be told apart on the way back — those are asserted
// separately below instead of expecting a self round-trip).
func TestAbstractType_RoundTrip(t *testing.T) {
	dialects := []string{"postgresql", "mysql", "sqlite"}
	abstractTypes := []struct {
		abstract        string
		length          int
		prec, scale     int
		sqliteCollision string // if non-empty, sqlite is expected to resolve to this instead
	}{
		{abstract: "string", length: 100},
		{abstract: "text"},
		{abstract: "integer"},
		{abstract: "bigint", sqliteCollision: "integer"},
		{abstract: "boolean"},
		{abstract: "uuid", sqliteCollision: "text"},
		{abstract: "timestamp"},
		{abstract: "date"},
		{abstract: "decimal", prec: 10, scale: 2},
		{abstract: "json", sqliteCollision: "text"},
		{abstract: "float"},
	}

	for _, dialect := range dialects {
		for _, at := range abstractTypes {
			native, err := sqlgen.NativeColumnType(dialect, at.abstract, at.length, at.prec, at.scale)
			if err != nil {
				t.Fatalf("NativeColumnType(%q, %q): %v", dialect, at.abstract, err)
			}

			gotAbstract, gotLen, gotPrec, gotScale, ok := sqlgen.AbstractType(dialect, native)
			if !ok {
				t.Errorf("AbstractType(%q, %q): expected a match, got none", dialect, native)
				continue
			}

			want := at.abstract
			if dialect == "sqlite" && at.sqliteCollision != "" {
				want = at.sqliteCollision
			}
			if gotAbstract != want {
				t.Errorf("AbstractType(%q, %q) = %q, want %q", dialect, native, gotAbstract, want)
			}

			switch at.abstract {
			case "string":
				if gotLen != at.length {
					t.Errorf("AbstractType(%q, %q) length = %d, want %d", dialect, native, gotLen, at.length)
				}
			case "decimal":
				if gotPrec != at.prec || gotScale != at.scale {
					t.Errorf("AbstractType(%q, %q) = (%d,%d), want (%d,%d)", dialect, native, gotPrec, gotScale, at.prec, at.scale)
				}
			}
		}
	}
}

func TestAbstractType_DefaultLength(t *testing.T) {
	abstract, length, _, _, ok := sqlgen.AbstractType("postgresql", "VARCHAR(255)")
	if !ok || abstract != "string" || length != 255 {
		t.Errorf("AbstractType(postgresql, VARCHAR(255)) = (%q, %d, ok=%v), want (string, 255, true)", abstract, length, ok)
	}
}

func TestAbstractType_CaseInsensitive(t *testing.T) {
	if _, _, _, _, ok := sqlgen.AbstractType("postgresql", "varchar(50)"); !ok {
		t.Error("expected lowercase native type to still match")
	}
}

// TestAbstractType_NoMatch documents the raw-native fallback path: a type
// with no abstract equivalent (a dialect-specific extension like Postgres's
// hstore, or a MySQL ENUM(...)) should report ok=false rather than
// mis-mapping to something close, so scanner callers know to preserve the
// raw native type string instead of guessing.
func TestAbstractType_NoMatch(t *testing.T) {
	cases := []struct {
		dialect string
		native  string
	}{
		{"postgresql", "hstore"},
		{"mysql", "ENUM('a','b')"},
		{"sqlite", "BLOB"},
		{"oracle", "VARCHAR(10)"}, // unknown dialect entirely
	}
	for _, c := range cases {
		if _, _, _, _, ok := sqlgen.AbstractType(c.dialect, c.native); ok {
			t.Errorf("AbstractType(%q, %q): expected ok=false, got true", c.dialect, c.native)
		}
	}
}
