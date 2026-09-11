package scanner

import (
	"database/sql"
	"testing"
)

// White-box tests for the unexported, DB-independent type/action-mapping
// helpers in postgres.go and mysql.go. These live alongside the source
// (package scanner) rather than in the tests/ tree because the functions
// under test are unexported pure functions with no way to reach them from a
// black-box _test package; the sqlmock-backed integration behavior of the
// scanners that call them is covered separately in tests/scanner/.

func nullInt64(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }

var invalidInt64 sql.NullInt64

func TestPgNativeType(t *testing.T) {
	tests := []struct {
		name                       string
		dataType                   string
		charLen, numPrec, numScale sql.NullInt64
		wantNative                 string
		wantOK                     bool
	}{
		{name: "varchar with length", dataType: "character varying", charLen: nullInt64(100), wantNative: "VARCHAR(100)", wantOK: true},
		{name: "varchar with NULL length defaults to 255", dataType: "character varying", charLen: invalidInt64, wantNative: "VARCHAR(255)", wantOK: true},
		{name: "text", dataType: "text", wantNative: "TEXT", wantOK: true},
		{name: "integer", dataType: "integer", wantNative: "INTEGER", wantOK: true},
		{name: "bigint", dataType: "bigint", wantNative: "BIGINT", wantOK: true},
		{name: "boolean", dataType: "boolean", wantNative: "BOOLEAN", wantOK: true},
		{name: "uuid", dataType: "uuid", wantNative: "UUID", wantOK: true},
		{name: "timestamp without time zone", dataType: "timestamp without time zone", wantNative: "TIMESTAMP", wantOK: true},
		{name: "timestamp with time zone", dataType: "timestamp with time zone", wantNative: "TIMESTAMP", wantOK: true},
		{name: "date", dataType: "date", wantNative: "DATE", wantOK: true},
		{name: "numeric with precision and scale", dataType: "numeric", numPrec: nullInt64(12), numScale: nullInt64(4), wantNative: "NUMERIC(12,4)", wantOK: true},
		{name: "numeric with NULL precision and scale defaults to 10,0", dataType: "numeric", numPrec: invalidInt64, numScale: invalidInt64, wantNative: "NUMERIC(10,0)", wantOK: true},
		{name: "numeric with only precision set", dataType: "numeric", numPrec: nullInt64(6), numScale: invalidInt64, wantNative: "NUMERIC(6,0)", wantOK: true},
		{name: "numeric with only scale set", dataType: "numeric", numPrec: invalidInt64, numScale: nullInt64(2), wantNative: "NUMERIC(10,2)", wantOK: true},
		{name: "jsonb", dataType: "jsonb", wantNative: "JSONB", wantOK: true},
		{name: "double precision", dataType: "double precision", wantNative: "DOUBLE PRECISION", wantOK: true},
		{name: "unrecognized postgres-specific type", dataType: "ARRAY", wantOK: false},
		{name: "user-defined domain/enum", dataType: "USER-DEFINED", wantOK: false},
		{name: "tsvector", dataType: "tsvector", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			native, ok := pgNativeType(tt.dataType, tt.charLen, tt.numPrec, tt.numScale)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && native != tt.wantNative {
				t.Errorf("native = %q, want %q", native, tt.wantNative)
			}
		})
	}
}

func TestPgConstraintAction(t *testing.T) {
	tests := []struct {
		name string
		c    rune
		want string
	}{
		{"restrict", 'r', "RESTRICT"},
		{"cascade", 'c', "CASCADE"},
		{"set null", 'n', "SET NULL"},
		{"set default", 'd', "SET DEFAULT"},
		{"no action (explicit default)", 'a', ""},
		{"unrecognized rune", 'x', ""},
		{"zero value rune", rune(0), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pgConstraintAction(tt.c); got != tt.want {
				t.Errorf("pgConstraintAction(%q) = %q, want %q", tt.c, got, tt.want)
			}
		})
	}
}

func TestPgCheckExpr(t *testing.T) {
	tests := []struct {
		name string
		def  string
		want string
	}{
		{"standard wrapper", "CHECK (price > 0)", "price > 0"},
		{"nested parens", "CHECK ((a > 0) AND (b < 10))", "(a > 0) AND (b < 10)"},
		{"no CHECK prefix", "price > 0", "price > 0"},
		{"malformed, no trailing paren", "CHECK (price > 0", "CHECK (price > 0"},
		{"empty string", "", ""},
		{"prefix without matching suffix content", "CHECK(x)", "CHECK(x)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pgCheckExpr(tt.def); got != tt.want {
				t.Errorf("pgCheckExpr(%q) = %q, want %q", tt.def, got, tt.want)
			}
		})
	}
}

func TestMysqlNativeType(t *testing.T) {
	tests := []struct {
		name                       string
		dataType, columnType       string
		charLen, numPrec, numScale sql.NullInt64
		wantNative                 string
		wantOK                     bool
	}{
		{name: "varchar with length", dataType: "varchar", columnType: "varchar(50)", charLen: nullInt64(50), wantNative: "VARCHAR(50)", wantOK: true},
		{name: "varchar with NULL length defaults to 255", dataType: "varchar", columnType: "varchar", charLen: invalidInt64, wantNative: "VARCHAR(255)", wantOK: true},
		{name: "text", dataType: "text", columnType: "text", wantNative: "TEXT", wantOK: true},
		{name: "int", dataType: "int", columnType: "int(11)", wantNative: "INT", wantOK: true},
		{name: "bigint", dataType: "bigint", columnType: "bigint(20)", wantNative: "BIGINT", wantOK: true},
		{name: "tinyint(1) maps to boolean-ish TINYINT(1)", dataType: "tinyint", columnType: "tinyint(1)", wantNative: "TINYINT(1)", wantOK: true},
		{name: "tinyint(1) unsigned still matches via prefix", dataType: "tinyint", columnType: "tinyint(1) unsigned", wantNative: "TINYINT(1)", wantOK: true},
		{name: "tinyint(4) does not map", dataType: "tinyint", columnType: "tinyint(4)", wantOK: false},
		{name: "char(36) maps to UUID-as-char", dataType: "char", columnType: "char(36)", wantNative: "CHAR(36)", wantOK: true},
		{name: "char(10) does not map (exact-equality, not prefix)", dataType: "char", columnType: "char(10)", wantOK: false},
		{name: "char(1) does not map", dataType: "char", columnType: "char(1)", wantOK: false},
		{name: "datetime", dataType: "datetime", columnType: "datetime", wantNative: "DATETIME", wantOK: true},
		{name: "date", dataType: "date", columnType: "date", wantNative: "DATE", wantOK: true},
		{name: "decimal with precision and scale", dataType: "decimal", columnType: "decimal(8,2)", numPrec: nullInt64(8), numScale: nullInt64(2), wantNative: "DECIMAL(8,2)", wantOK: true},
		{name: "decimal with NULL precision/scale defaults to 10,0", dataType: "decimal", columnType: "decimal", numPrec: invalidInt64, numScale: invalidInt64, wantNative: "DECIMAL(10,0)", wantOK: true},
		{name: "json", dataType: "json", columnType: "json", wantNative: "JSON", wantOK: true},
		{name: "double", dataType: "double", columnType: "double", wantNative: "DOUBLE", wantOK: true},
		{name: "enum is unmappable", dataType: "enum", columnType: "enum('a','b')", wantOK: false},
		{name: "set is unmappable", dataType: "set", columnType: "set('a','b')", wantOK: false},
		{name: "blob is unmappable", dataType: "blob", columnType: "blob", wantOK: false},
		{name: "mediumtext is unmappable", dataType: "mediumtext", columnType: "mediumtext", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			native, ok := mysqlNativeType(tt.dataType, tt.columnType, tt.charLen, tt.numPrec, tt.numScale)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && native != tt.wantNative {
				t.Errorf("native = %q, want %q", native, tt.wantNative)
			}
		})
	}
}

func TestMysqlReferentialAction(t *testing.T) {
	tests := []struct {
		name string
		rule string
		want string
	}{
		{"empty string", "", ""},
		{"exact NO ACTION", "NO ACTION", ""},
		{"lowercase no action", "no action", ""},
		{"mixed case No Action", "No Action", ""},
		{"NO ACTION with trailing space is NOT trimmed/folded", "NO ACTION ", "NO ACTION "},
		{"cascade passthrough, uppercase preserved", "CASCADE", "CASCADE"},
		{"cascade passthrough, original lowercase preserved (no forced uppercasing)", "cascade", "cascade"},
		{"set null passthrough", "SET NULL", "SET NULL"},
		{"set default passthrough", "SET DEFAULT", "SET DEFAULT"},
		{"restrict passthrough", "RESTRICT", "RESTRICT"},
		{"unrecognized rule passed through verbatim", "SOME FUTURE RULE", "SOME FUTURE RULE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mysqlReferentialAction(tt.rule); got != tt.want {
				t.Errorf("mysqlReferentialAction(%q) = %q, want %q", tt.rule, got, tt.want)
			}
		})
	}
}
