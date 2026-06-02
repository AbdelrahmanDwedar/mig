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
	if err := migrator.Migrate(); err != nil {
		t.Fatal(err)
	}
	if len(driver.AppliedMigrations) != 1 || driver.AppliedMigrations[0] != mName {
		t.Error("Migration not applied correctly")
	}

	// Test Rollback
	if err := migrator.Rollback(1, ""); err != nil {
		t.Fatal(err)
	}
	if len(driver.AppliedMigrations) != 0 {
		t.Error("Rollback failed")
	}
}
