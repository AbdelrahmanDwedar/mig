package sqlgen_test

import (
	"encoding/json"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

func TestSQLite_CreateTable_AutoIncrementPK(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{
		"op": "create_table",
		"table": "users",
		"columns": [
			{"name": "id", "type": "integer", "auto_increment": true, "primary_key": true},
			{"name": "email", "type": "string", "length": 255, "nullable": false, "unique": true}
		]
	}`)
	assertStatements(t, stmts, []string{
		"CREATE TABLE \"users\" (\n  \"id\" INTEGER PRIMARY KEY AUTOINCREMENT,\n  \"email\" VARCHAR(255) NOT NULL UNIQUE\n)",
	})
}

func TestSQLite_CreateTable_CompositePK(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{
		"op": "create_table",
		"table": "role_users",
		"columns": [
			{"name": "role_id", "type": "integer", "primary_key": true},
			{"name": "user_id", "type": "integer", "primary_key": true}
		]
	}`)
	assertStatements(t, stmts, []string{
		"CREATE TABLE \"role_users\" (\n  \"role_id\" INTEGER,\n  \"user_id\" INTEGER,\n  PRIMARY KEY (\"role_id\", \"user_id\")\n)",
	})
}

func TestSQLite_AutoIncrement_CompositePK_Rejected(t *testing.T) {
	d := mustDialect(t, "sqlite")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{
		"op": "create_table",
		"table": "role_users",
		"columns": [
			{"name": "role_id", "type": "integer", "primary_key": true, "auto_increment": true},
			{"name": "user_id", "type": "integer", "primary_key": true}
		]
	}`))
	if err == nil {
		t.Fatal("expected error for sqlite auto_increment on a composite primary key, got nil")
	}
}

func TestSQLite_AutoIncrement_InvalidType(t *testing.T) {
	d := mustDialect(t, "sqlite")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "create_table", "table": "t", "columns": [{"name": "id", "type": "string", "auto_increment": true, "primary_key": true}]}`))
	if err == nil {
		t.Fatal("expected error for sqlite auto_increment on a non-integer column, got nil")
	}
}

func TestSQLite_DropTable(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{"op": "drop_table", "table": "users", "if_exists": true}`)
	assertStatements(t, stmts, []string{`DROP TABLE IF EXISTS "users"`})
}

func TestSQLite_RenameTable(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{"op": "rename_table", "from": "old_name", "to": "new_name"}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "old_name" RENAME TO "new_name"`})
}

func TestSQLite_AddColumn(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{"op": "add_column", "table": "organizations", "column": {"name": "plan", "type": "string", "length": 50, "default": {"literal": "free"}}}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "organizations" ADD COLUMN "plan" VARCHAR(50) DEFAULT 'free'`})
}

func TestSQLite_DropColumn(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{"op": "drop_column", "table": "organizations", "column": "plan"}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "organizations" DROP COLUMN "plan"`})
}

func TestSQLite_RenameColumn(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{"op": "rename_column", "table": "users", "from": "email", "to": "email_address"}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "users" RENAME COLUMN "email" TO "email_address"`})
}

func TestSQLite_AddIndex(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{"op": "add_index", "table": "users", "index": {"name": "idx_users_created_at", "columns": ["created_at"]}}`)
	assertStatements(t, stmts, []string{`CREATE INDEX "idx_users_created_at" ON "users" ("created_at")`})
}

func TestSQLite_DropIndex(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{"op": "drop_index", "table": "users", "name": "idx_users_created_at"}`)
	assertStatements(t, stmts, []string{`DROP INDEX "idx_users_created_at"`})
}

func TestSQLite_AlterColumn_Unsupported(t *testing.T) {
	d := mustDialect(t, "sqlite")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "alter_column", "table": "users", "column": "age", "type": "bigint"}`))
	if err == nil {
		t.Fatal("expected error for sqlite alter_column, got nil")
	}
}

func TestSQLite_AddForeignKey_Unsupported(t *testing.T) {
	d := mustDialect(t, "sqlite")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "add_foreign_key", "table": "posts", "foreign_key": {"columns": ["author_id"], "ref_table": "users", "ref_columns": ["id"]}}`))
	if err == nil {
		t.Fatal("expected error for sqlite add_foreign_key, got nil")
	}
}

func TestSQLite_DropForeignKey_Unsupported(t *testing.T) {
	d := mustDialect(t, "sqlite")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "drop_foreign_key", "table": "posts", "name": "fk_posts_author"}`))
	if err == nil {
		t.Fatal("expected error for sqlite drop_foreign_key, got nil")
	}
}

func TestSQLite_AddConstraint_Unsupported(t *testing.T) {
	d := mustDialect(t, "sqlite")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "add_constraint", "table": "users", "check": {"expr": "age >= 0"}}`))
	if err == nil {
		t.Fatal("expected error for sqlite add_constraint, got nil")
	}
}

func TestSQLite_DropConstraint_Unsupported(t *testing.T) {
	d := mustDialect(t, "sqlite")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "drop_constraint", "table": "users", "name": "chk_users_age_positive"}`))
	if err == nil {
		t.Fatal("expected error for sqlite drop_constraint, got nil")
	}
}

func TestSQLite_SQLEscapeHatch(t *testing.T) {
	d := mustDialect(t, "sqlite")
	stmts := buildOp(t, d, `{"op": "sql", "query": "CREATE VIEW active_users AS SELECT * FROM users WHERE is_active = 1"}`)
	assertStatements(t, stmts, []string{"CREATE VIEW active_users AS SELECT * FROM users WHERE is_active = 1"})
}
