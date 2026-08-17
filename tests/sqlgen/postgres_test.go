package sqlgen_test

import (
	"encoding/json"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

func mustDialect(t *testing.T, driver string) sqlgen.Dialect {
	t.Helper()
	d, err := sqlgen.New(driver)
	if err != nil {
		t.Fatalf("sqlgen.New(%q): %v", driver, err)
	}
	return d
}

func buildOp(t *testing.T, dialect sqlgen.Dialect, opJSON string) []string {
	t.Helper()
	stmts, err := sqlgen.BuildStatement(dialect, json.RawMessage(opJSON))
	if err != nil {
		t.Fatalf("BuildStatement(%s): unexpected error: %v", opJSON, err)
	}
	return stmts
}

func assertStatements(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d statements, want %d\ngot:  %#v\nwant: %#v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("statement %d:\ngot:  %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestPostgres_CreateTable(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{
		"op": "create_table",
		"table": "users",
		"columns": [
			{"name": "id", "type": "integer", "auto_increment": true, "primary_key": true},
			{"name": "email", "type": "string", "length": 255, "nullable": false, "unique": true},
			{"name": "is_active", "type": "boolean", "nullable": false, "default": {"literal": true}}
		],
		"indexes": [{"name": "idx_users_email", "columns": ["email"], "unique": true}]
	}`)
	assertStatements(t, stmts, []string{
		"CREATE TABLE \"users\" (\n  \"id\" SERIAL,\n  \"email\" VARCHAR(255) NOT NULL UNIQUE,\n  \"is_active\" BOOLEAN NOT NULL DEFAULT TRUE,\n  PRIMARY KEY (\"id\")\n)",
		`CREATE UNIQUE INDEX "idx_users_email" ON "users" ("email")`,
	})
}

func TestPostgres_CreateTable_CompositePK(t *testing.T) {
	d := mustDialect(t, "postgresql")
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

func TestPostgres_CreateTable_ForeignKeyAndCheck(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{
		"op": "create_table",
		"table": "posts",
		"columns": [{"name": "author_id", "type": "integer"}],
		"foreign_keys": [{"name": "fk_posts_author", "columns": ["author_id"], "ref_table": "users", "ref_columns": ["id"], "on_delete": "cascade"}],
		"checks": [{"name": "chk_posts_author_positive", "expr": "author_id >= 0"}]
	}`)
	assertStatements(t, stmts, []string{
		"CREATE TABLE \"posts\" (\n  \"author_id\" INTEGER,\n  CONSTRAINT \"fk_posts_author\" FOREIGN KEY (\"author_id\") REFERENCES \"users\" (\"id\") ON DELETE CASCADE,\n  CONSTRAINT \"chk_posts_author_positive\" CHECK (author_id >= 0)\n)",
	})
}

func TestPostgres_DropTable(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "drop_table", "table": "users", "if_exists": true}`)
	assertStatements(t, stmts, []string{`DROP TABLE IF EXISTS "users"`})
}

func TestPostgres_RenameTable(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "rename_table", "from": "old_name", "to": "new_name"}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "old_name" RENAME TO "new_name"`})
}

func TestPostgres_AddColumn(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "add_column", "table": "organizations", "column": {"name": "plan", "type": "string", "length": 50, "default": {"literal": "free"}}}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "organizations" ADD COLUMN "plan" VARCHAR(50) DEFAULT 'free'`})
}

func TestPostgres_DropColumn(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "drop_column", "table": "organizations", "column": "plan"}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "organizations" DROP COLUMN "plan"`})
}

func TestPostgres_RenameColumn(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "rename_column", "table": "users", "from": "email", "to": "email_address"}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "users" RENAME COLUMN "email" TO "email_address"`})
}

func TestPostgres_AlterColumn(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "alter_column", "table": "users", "column": "age", "type": "bigint", "nullable": false, "default": {"literal": 0}}`)
	assertStatements(t, stmts, []string{
		`ALTER TABLE "users" ALTER COLUMN "age" TYPE BIGINT, ALTER COLUMN "age" SET NOT NULL, ALTER COLUMN "age" SET DEFAULT 0`,
	})
}

func TestPostgres_CreateTable_BigintAutoIncrement(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{
		"op": "create_table",
		"table": "t",
		"columns": [{"name": "id", "type": "bigint", "auto_increment": true, "primary_key": true}]
	}`)
	assertStatements(t, stmts, []string{
		"CREATE TABLE \"t\" (\n  \"id\" BIGSERIAL,\n  PRIMARY KEY (\"id\")\n)",
	})
}

func TestPostgres_AlterColumn_DropNotNull(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "alter_column", "table": "users", "column": "age", "nullable": true}`)
	assertStatements(t, stmts, []string{
		`ALTER TABLE "users" ALTER COLUMN "age" DROP NOT NULL`,
	})
}

func TestPostgres_AlterColumn_NoChanges(t *testing.T) {
	d := mustDialect(t, "postgresql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "alter_column", "table": "users", "column": "age"}`))
	if err == nil {
		t.Fatal("expected error for alter_column with no changes, got nil")
	}
}

func TestPostgres_AddIndex(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "add_index", "table": "users", "index": {"name": "idx_users_created_at", "columns": ["created_at"]}}`)
	assertStatements(t, stmts, []string{`CREATE INDEX "idx_users_created_at" ON "users" ("created_at")`})
}

func TestPostgres_DropIndex(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "drop_index", "table": "users", "name": "idx_users_created_at"}`)
	assertStatements(t, stmts, []string{`DROP INDEX "idx_users_created_at"`})
}

func TestPostgres_AddForeignKey(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "add_foreign_key", "table": "posts", "foreign_key": {"name": "fk_posts_author", "columns": ["author_id"], "ref_table": "users", "ref_columns": ["id"]}}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "posts" ADD CONSTRAINT "fk_posts_author" FOREIGN KEY ("author_id") REFERENCES "users" ("id")`})
}

func TestPostgres_DropForeignKey(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "drop_foreign_key", "table": "posts", "name": "fk_posts_author"}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "posts" DROP CONSTRAINT "fk_posts_author"`})
}

func TestPostgres_AddConstraint(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "add_constraint", "table": "users", "check": {"name": "chk_users_age_positive", "expr": "age >= 0"}}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "users" ADD CONSTRAINT "chk_users_age_positive" CHECK (age >= 0)`})
}

func TestPostgres_DropConstraint(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "drop_constraint", "table": "users", "name": "chk_users_age_positive"}`)
	assertStatements(t, stmts, []string{`ALTER TABLE "users" DROP CONSTRAINT "chk_users_age_positive"`})
}

func TestPostgres_SQLEscapeHatch(t *testing.T) {
	d := mustDialect(t, "postgresql")
	stmts := buildOp(t, d, `{"op": "sql", "query": "CREATE MATERIALIZED VIEW active_users AS SELECT * FROM users WHERE is_active = TRUE"}`)
	assertStatements(t, stmts, []string{"CREATE MATERIALIZED VIEW active_users AS SELECT * FROM users WHERE is_active = TRUE"})
}

func TestPostgres_AutoIncrement_InvalidType(t *testing.T) {
	d := mustDialect(t, "postgresql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "create_table", "table": "t", "columns": [{"name": "id", "type": "string", "auto_increment": true}]}`))
	if err == nil {
		t.Fatal("expected error for auto_increment on non-integer type, got nil")
	}
}

func TestBuildStatement_UnknownOp(t *testing.T) {
	d := mustDialect(t, "postgresql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "frobnicate_table"}`))
	if err == nil {
		t.Fatal("expected error for unknown op, got nil")
	}
}
