package migrate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/db"
	"github.com/AbdelrahmanDwedar/mig/internal/migrate"
	"github.com/AbdelrahmanDwedar/mig/internal/parser"
)

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
		Driver: driver,
		Parser: &parser.SQLParser{},
		Dir:    tmpDir,
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
