package db_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/config"
	"github.com/AbdelrahmanDwedar/mig/internal/db"
	"github.com/AbdelrahmanDwedar/mig/internal/parser"
	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteDriver_Integration(t *testing.T) {
	dbName := "test_integration.db"
	defer os.Remove(dbName)

	cfg := &config.DatabaseConfig{Driver: "sqlite", DBName: dbName}
	driver, err := db.NewDriver(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := driver.Connect(); err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.EnsureMigrationsTable(); err != nil {
		t.Fatal(err)
	}

	mName := "2026_01_01_test.sql"
	if err := driver.ApplyMigration(mName, "CREATE TABLE users (id INT)"); err != nil {
		t.Fatal(err)
	}

	applied, err := driver.GetAppliedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || applied[0] != mName {
		t.Error("Migration not found in tracking table")
	}

	if err := driver.RollbackMigration(mName, "DROP TABLE users"); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteDriver_ApplyMigration_BadSQL(t *testing.T) {
	dbName := "test_apply_bad_sql.db"
	defer os.Remove(dbName)

	cfg := &config.DatabaseConfig{Driver: "sqlite", DBName: dbName}
	driver, err := db.NewDriver(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.Connect(); err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.EnsureMigrationsTable(); err != nil {
		t.Fatal(err)
	}

	if err := driver.ApplyMigration("bad.sql", "NOT VALID SQL;"); err == nil {
		t.Fatal("expected error for invalid SQL, got nil")
	}

	applied, err := driver.GetAppliedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 0 {
		t.Error("failed migration should not be recorded as applied")
	}
}

func TestSQLiteDriver_RollbackMigration_BadSQL(t *testing.T) {
	dbName := "test_rollback_bad_sql.db"
	defer os.Remove(dbName)

	cfg := &config.DatabaseConfig{Driver: "sqlite", DBName: dbName}
	driver, err := db.NewDriver(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.Connect(); err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.EnsureMigrationsTable(); err != nil {
		t.Fatal(err)
	}

	mName := "2026_01_01_test.sql"
	if err := driver.ApplyMigration(mName, "CREATE TABLE users (id INT)"); err != nil {
		t.Fatal(err)
	}

	if err := driver.RollbackMigration(mName, "NOT VALID SQL;"); err == nil {
		t.Fatal("expected error for invalid down SQL, got nil")
	}

	applied, err := driver.GetAppliedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 {
		t.Error("failed rollback should leave the migration recorded as applied")
	}
}

func TestSQLiteDriver_JSONMigration_RoundTrip(t *testing.T) {
	dbName := "test_json_roundtrip.db"
	defer os.Remove(dbName)

	cfg := &config.DatabaseConfig{Driver: "sqlite", DBName: dbName}
	driver, err := db.NewDriver(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.Connect(); err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.EnsureMigrationsTable(); err != nil {
		t.Fatal(err)
	}

	dialect, err := sqlgen.New("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	jsonParser := parser.NewJSONParser(dialect)

	migrationJSON := `{
		"up": [
			{
				"op": "create_table",
				"table": "widgets",
				"columns": [
					{"name": "id", "type": "integer", "auto_increment": true, "primary_key": true},
					{"name": "name", "type": "string", "length": 100, "nullable": false}
				],
				"indexes": [{"name": "idx_widgets_name", "columns": ["name"], "unique": true}]
			}
		],
		"down": [
			{"op": "drop_table", "table": "widgets", "if_exists": true}
		]
	}`

	up, down, err := jsonParser.Parse(migrationJSON)
	if err != nil {
		t.Fatal(err)
	}

	mName := "2026_01_01_000000_create_widgets.json"
	if err := driver.ApplyMigration(mName, up); err != nil {
		t.Fatalf("ApplyMigration: %v", err)
	}

	raw, err := sql.Open("sqlite3", dbName)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()

	rows, err := raw.Query("PRAGMA table_info(widgets)")
	if err != nil {
		t.Fatal(err)
	}
	var colNames []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		colNames = append(colNames, name)
	}
	rows.Close()
	if len(colNames) != 2 || colNames[0] != "id" || colNames[1] != "name" {
		t.Fatalf("expected columns [id, name], got %v", colNames)
	}

	idxRows, err := raw.Query("PRAGMA index_list(widgets)")
	if err != nil {
		t.Fatal(err)
	}
	var idxCount int
	for idxRows.Next() {
		idxCount++
	}
	idxRows.Close()
	if idxCount != 1 {
		t.Fatalf("expected 1 index, got %d", idxCount)
	}

	if err := driver.RollbackMigration(mName, down); err != nil {
		t.Fatalf("RollbackMigration: %v", err)
	}

	tableRows, err := raw.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='widgets'")
	if err != nil {
		t.Fatal(err)
	}
	if tableRows.Next() {
		t.Error("expected widgets table to be dropped after rollback")
	}
	tableRows.Close()
}
