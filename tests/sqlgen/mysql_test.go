package sqlgen_test

import (
	"encoding/json"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

func TestMySQL_CreateTable(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{
		"op": "create_table",
		"table": "users",
		"columns": [
			{"name": "id", "type": "integer", "auto_increment": true, "primary_key": true},
			{"name": "email", "type": "string", "length": 255, "nullable": false, "unique": true}
		]
	}`)
	assertStatements(t, stmts, []string{
		"CREATE TABLE `users` (\n  `id` INT AUTO_INCREMENT,\n  `email` VARCHAR(255) NOT NULL UNIQUE,\n  PRIMARY KEY (`id`)\n)",
	})
}

func TestMySQL_DropTable(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "drop_table", "table": "users", "if_exists": true}`)
	assertStatements(t, stmts, []string{"DROP TABLE IF EXISTS `users`"})
}

func TestMySQL_RenameTable(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "rename_table", "from": "old_name", "to": "new_name"}`)
	assertStatements(t, stmts, []string{"ALTER TABLE `old_name` RENAME TO `new_name`"})
}

func TestMySQL_AddColumn(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "add_column", "table": "organizations", "column": {"name": "plan", "type": "string", "length": 50, "default": {"literal": "free"}}}`)
	assertStatements(t, stmts, []string{"ALTER TABLE `organizations` ADD COLUMN `plan` VARCHAR(50) DEFAULT 'free'"})
}

func TestMySQL_DropColumn(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "drop_column", "table": "organizations", "column": "plan"}`)
	assertStatements(t, stmts, []string{"ALTER TABLE `organizations` DROP COLUMN `plan`"})
}

func TestMySQL_RenameColumn(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "rename_column", "table": "users", "from": "email", "to": "email_address"}`)
	assertStatements(t, stmts, []string{"ALTER TABLE `users` RENAME COLUMN `email` TO `email_address`"})
}

func TestMySQL_AlterColumn(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "alter_column", "table": "users", "column": "age", "type": "bigint", "nullable": false, "default": {"literal": 0}}`)
	assertStatements(t, stmts, []string{"ALTER TABLE `users` MODIFY COLUMN `age` BIGINT NOT NULL DEFAULT 0"})
}

func TestMySQL_CreateTable_BigintAutoIncrement(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{
		"op": "create_table",
		"table": "t",
		"columns": [{"name": "id", "type": "bigint", "auto_increment": true, "primary_key": true}]
	}`)
	assertStatements(t, stmts, []string{
		"CREATE TABLE `t` (\n  `id` BIGINT AUTO_INCREMENT,\n  PRIMARY KEY (`id`)\n)",
	})
}

func TestMySQL_AlterColumn_InvalidType(t *testing.T) {
	d := mustDialect(t, "mysql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "alter_column", "table": "users", "column": "age", "type": "unobtainium"}`))
	if err == nil {
		t.Fatal("expected error for unsupported alter_column type, got nil")
	}
}

func TestMySQL_AlterColumn_RequiresType(t *testing.T) {
	d := mustDialect(t, "mysql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "alter_column", "table": "users", "column": "age", "nullable": false}`))
	if err == nil {
		t.Fatal("expected error for mysql alter_column without type, got nil")
	}
}

func TestMySQL_AddIndex(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "add_index", "table": "users", "index": {"name": "idx_users_created_at", "columns": ["created_at"]}}`)
	assertStatements(t, stmts, []string{"CREATE INDEX `idx_users_created_at` ON `users` (`created_at`)"})
}

func TestMySQL_DropIndex(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "drop_index", "table": "users", "name": "idx_users_created_at"}`)
	assertStatements(t, stmts, []string{"DROP INDEX `idx_users_created_at` ON `users`"})
}

func TestMySQL_AddForeignKey(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "add_foreign_key", "table": "posts", "foreign_key": {"name": "fk_posts_author", "columns": ["author_id"], "ref_table": "users", "ref_columns": ["id"]}}`)
	assertStatements(t, stmts, []string{"ALTER TABLE `posts` ADD CONSTRAINT `fk_posts_author` FOREIGN KEY (`author_id`) REFERENCES `users` (`id`)"})
}

func TestMySQL_DropForeignKey(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "drop_foreign_key", "table": "posts", "name": "fk_posts_author"}`)
	assertStatements(t, stmts, []string{"ALTER TABLE `posts` DROP FOREIGN KEY `fk_posts_author`"})
}

func TestMySQL_AddConstraint(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "add_constraint", "table": "users", "check": {"name": "chk_users_age_positive", "expr": "age >= 0"}}`)
	assertStatements(t, stmts, []string{"ALTER TABLE `users` ADD CONSTRAINT `chk_users_age_positive` CHECK (age >= 0)"})
}

func TestMySQL_DropConstraint(t *testing.T) {
	d := mustDialect(t, "mysql")
	stmts := buildOp(t, d, `{"op": "drop_constraint", "table": "users", "name": "chk_users_age_positive"}`)
	assertStatements(t, stmts, []string{"ALTER TABLE `users` DROP CHECK `chk_users_age_positive`"})
}

func TestMySQL_AutoIncrement_InvalidType(t *testing.T) {
	d := mustDialect(t, "mysql")
	_, err := sqlgen.BuildStatement(d, json.RawMessage(`{"op": "create_table", "table": "t", "columns": [{"name": "id", "type": "string", "auto_increment": true}]}`))
	if err == nil {
		t.Fatal("expected error for auto_increment on non-integer type, got nil")
	}
}
