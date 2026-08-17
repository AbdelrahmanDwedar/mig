package sqlgen_test

import (
	"encoding/json"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

func TestNew_UnsupportedDriver(t *testing.T) {
	if _, err := sqlgen.New("oracle"); err == nil {
		t.Fatal("expected error for unsupported driver, got nil")
	}
}

func TestDialect_Name(t *testing.T) {
	tests := []struct {
		driver string
		want   string
	}{
		{"postgresql", "postgresql"},
		{"mysql", "mysql"},
		{"sqlite", "sqlite"},
	}
	for _, tt := range tests {
		d := mustDialect(t, tt.driver)
		if got := d.Name(); got != tt.want {
			t.Errorf("Name() = %q, want %q", got, tt.want)
		}
	}
}

func TestBuildStatement_MissingOpField(t *testing.T) {
	d := mustDialect(t, "postgresql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"table": "users"}`))
	if err == nil {
		t.Fatal("expected error for missing \"op\" field, got nil")
	}
}

func TestBuildStatement_InvalidOpPayload(t *testing.T) {
	d := mustDialect(t, "postgresql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "create_table", "table": "users", "columns": "not-an-array"}`))
	if err == nil {
		t.Fatal("expected error for invalid op payload, got nil")
	}
}

func TestBuildStatement_InvalidEnvelope(t *testing.T) {
	d := mustDialect(t, "postgresql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON envelope, got nil")
	}
}

func TestAddColumn_UnsupportedType(t *testing.T) {
	for _, driver := range []string{"postgresql", "mysql", "sqlite"} {
		t.Run(driver, func(t *testing.T) {
			d := mustDialect(t, driver)
			_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "add_column", "table": "t", "column": {"name": "c", "type": "unobtainium"}}`))
			if err == nil {
				t.Fatal("expected error for unsupported column type, got nil")
			}
		})
	}
}

func TestAddForeignKey_OnUpdate(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "add_foreign_key", "table": "posts", "foreign_key": {"name": "fk_posts_author", "columns": ["author_id"], "ref_table": "users", "ref_columns": ["id"], "on_update": "cascade"}}`)
	assertStatements(t, stmts, []string{
		`ALTER TABLE "posts" ADD CONSTRAINT "fk_posts_author" FOREIGN KEY ("author_id") REFERENCES "users" ("id") ON UPDATE CASCADE`,
	})
}

func TestCreateTable_UnsupportedDefaultLiteralType(t *testing.T) {
	d := mustDialect(t, "postgresql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{
		"op": "create_table",
		"table": "t",
		"columns": [{"name": "c", "type": "string", "default": {"literal": [1, 2]}}]
	}`))
	if err == nil {
		t.Fatal("expected error for unsupported default literal type, got nil")
	}
}
