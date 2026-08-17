package migrate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/db"
	"github.com/AbdelrahmanDwedar/mig/internal/migrate"
	"github.com/AbdelrahmanDwedar/mig/internal/parser"
	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

func newSQLiteRegistry(t *testing.T) *parser.Registry {
	t.Helper()
	dialect, err := sqlgen.New("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	return parser.NewRegistry(dialect)
}

func TestMigrator_Flow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migrations")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mName := "2026_01_01_000000_test.sql"
	content := "-- +migrate Up\nUP\n-- +migrate Down\nDOWN"
	os.WriteFile(filepath.Join(tmpDir, mName), []byte(content), 0644)

	driver := &db.MockDriver{}
	migrator := &migrate.Migrator{
		Driver:   driver,
		Registry: newSQLiteRegistry(t),
		Dir:      tmpDir,
	}

	// Test Migrate
	applied, err := migrator.Migrate()
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || applied[0] != mName {
		t.Error("Migrate did not return the applied migration name")
	}
	if len(driver.AppliedMigrations) != 1 || driver.AppliedMigrations[0] != mName {
		t.Error("Migration not applied correctly")
	}

	// Test Rollback
	rolledBack, err := migrator.Rollback(1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rolledBack) != 1 || rolledBack[0] != mName {
		t.Error("Rollback did not return the rolled back migration name")
	}
	if len(driver.AppliedMigrations) != 0 {
		t.Error("Rollback failed")
	}
}

func TestMigrator_MixedSQLAndJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migrations")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sqlName := "2026_01_01_000000_create_orgs.sql"
	sqlContent := "-- +migrate Up\nCREATE TABLE organizations (id INTEGER);\n-- +migrate Down\nDROP TABLE organizations;"
	if err := os.WriteFile(filepath.Join(tmpDir, sqlName), []byte(sqlContent), 0644); err != nil {
		t.Fatal(err)
	}

	jsonName := "2026_01_02_000000_create_users.json"
	jsonContent := `{
		"up": [{"op": "create_table", "table": "users", "columns": [{"name": "id", "type": "integer"}]}],
		"down": [{"op": "drop_table", "table": "users"}]
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, jsonName), []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	driver := &db.MockDriver{}
	migrator := &migrate.Migrator{
		Driver:   driver,
		Registry: newSQLiteRegistry(t),
		Dir:      tmpDir,
	}

	if _, err := migrator.Migrate(); err != nil {
		t.Fatal(err)
	}
	if len(driver.AppliedMigrations) != 2 {
		t.Fatalf("expected 2 migrations applied, got %d", len(driver.AppliedMigrations))
	}
	if driver.AppliedMigrations[0] != sqlName || driver.AppliedMigrations[1] != jsonName {
		t.Errorf("expected chronological order [%s, %s], got %v", sqlName, jsonName, driver.AppliedMigrations)
	}

	status, err := migrator.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 2 {
		t.Fatalf("expected 2 entries in status, got %d", len(status))
	}
}
