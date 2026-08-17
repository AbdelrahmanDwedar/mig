package migrate_test

import (
	"errors"
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

// errDriver wraps a db.MockDriver and lets tests force specific methods to
// fail, to exercise Migrator's error-propagation branches without a real
// database.
type errDriver struct {
	db.MockDriver
	failEnsure   error
	failApplied  error
	failApply    error
	failRollback error
}

func (d *errDriver) EnsureMigrationsTable() error {
	if d.failEnsure != nil {
		return d.failEnsure
	}
	return d.MockDriver.EnsureMigrationsTable()
}

func (d *errDriver) GetAppliedMigrations() ([]string, error) {
	if d.failApplied != nil {
		return nil, d.failApplied
	}
	return d.MockDriver.GetAppliedMigrations()
}

func (d *errDriver) ApplyMigration(name, upSQL string) error {
	if d.failApply != nil {
		return d.failApply
	}
	return d.MockDriver.ApplyMigration(name, upSQL)
}

func (d *errDriver) RollbackMigration(name, downSQL string) error {
	if d.failRollback != nil {
		return d.failRollback
	}
	return d.MockDriver.RollbackMigration(name, downSQL)
}

func writeSQLMigration(t *testing.T, dir, name string) {
	t.Helper()
	content := "-- +migrate Up\nUP\n-- +migrate Down\nDOWN"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrator_Migrate_ErrorPaths(t *testing.T) {
	t.Run("EnsureMigrationsTable error", func(t *testing.T) {
		tmpDir := t.TempDir()
		driver := &errDriver{failEnsure: errors.New("boom")}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: tmpDir}
		if _, err := migrator.Migrate(); err == nil {
			t.Fatal("expected error from EnsureMigrationsTable, got nil")
		}
	})

	t.Run("GetAppliedMigrations error", func(t *testing.T) {
		tmpDir := t.TempDir()
		driver := &errDriver{failApplied: errors.New("boom")}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: tmpDir}
		if _, err := migrator.Migrate(); err == nil {
			t.Fatal("expected error from GetAppliedMigrations, got nil")
		}
	})

	t.Run("ReadDir error", func(t *testing.T) {
		driver := &errDriver{}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: "/nonexistent/path/for/mig/tests"}
		if _, err := migrator.Migrate(); err == nil {
			t.Fatal("expected error from os.ReadDir on a nonexistent dir, got nil")
		}
	})

	t.Run("ApplyMigration error", func(t *testing.T) {
		tmpDir := t.TempDir()
		writeSQLMigration(t, tmpDir, "2026_01_01_000000_test.sql")
		driver := &errDriver{failApply: errors.New("boom")}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: tmpDir}
		applied, err := migrator.Migrate()
		if err == nil {
			t.Fatal("expected error from ApplyMigration, got nil")
		}
		if len(applied) != 0 {
			t.Errorf("expected no migrations recorded as applied, got %v", applied)
		}
	})
}

func TestMigrator_Rollback_ErrorPaths(t *testing.T) {
	t.Run("GetAppliedMigrations error", func(t *testing.T) {
		driver := &errDriver{failApplied: errors.New("boom")}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: t.TempDir()}
		if _, err := migrator.Rollback(1, ""); err == nil {
			t.Fatal("expected error from GetAppliedMigrations, got nil")
		}
	})

	t.Run("no migrations applied", func(t *testing.T) {
		driver := &db.MockDriver{}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: t.TempDir()}
		if _, err := migrator.Rollback(1, ""); err == nil {
			t.Fatal("expected error when nothing is applied, got nil")
		}
	})

	t.Run("migration path not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		writeSQLMigration(t, tmpDir, "2026_01_01_000000_test.sql")
		driver := &db.MockDriver{}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: tmpDir}
		if _, err := migrator.Migrate(); err != nil {
			t.Fatal(err)
		}
		if _, err := migrator.Rollback(0, "does_not_match"); err == nil {
			t.Fatal("expected error for unmatched --migration path, got nil")
		}
	})

	t.Run("RollbackMigration error", func(t *testing.T) {
		tmpDir := t.TempDir()
		writeSQLMigration(t, tmpDir, "2026_01_01_000000_test.sql")
		driver := &errDriver{}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: tmpDir}
		if _, err := migrator.Migrate(); err != nil {
			t.Fatal(err)
		}
		driver.failRollback = errors.New("boom")
		rolledBack, err := migrator.Rollback(1, "")
		if err == nil {
			t.Fatal("expected error from RollbackMigration, got nil")
		}
		if len(rolledBack) != 0 {
			t.Errorf("expected no migrations recorded as rolled back, got %v", rolledBack)
		}
	})

	t.Run("ReadFile error", func(t *testing.T) {
		driver := &db.MockDriver{AppliedMigrations: []string{"missing.sql"}}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: t.TempDir()}
		if _, err := migrator.Rollback(1, ""); err == nil {
			t.Fatal("expected error reading a migration file that no longer exists on disk, got nil")
		}
	})
}

func TestMigrator_Reset_ErrorPaths(t *testing.T) {
	t.Run("GetAppliedMigrations error", func(t *testing.T) {
		driver := &errDriver{failApplied: errors.New("boom")}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: t.TempDir()}
		if _, err := migrator.Reset(); err == nil {
			t.Fatal("expected error from GetAppliedMigrations, got nil")
		}
	})

	t.Run("RollbackMigration error", func(t *testing.T) {
		tmpDir := t.TempDir()
		writeSQLMigration(t, tmpDir, "2026_01_01_000000_test.sql")
		driver := &errDriver{}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: tmpDir}
		if _, err := migrator.Migrate(); err != nil {
			t.Fatal(err)
		}
		driver.failRollback = errors.New("boom")
		if _, err := migrator.Reset(); err == nil {
			t.Fatal("expected error from RollbackMigration, got nil")
		}
	})

	t.Run("ReadFile error", func(t *testing.T) {
		driver := &db.MockDriver{AppliedMigrations: []string{"missing.sql"}}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: t.TempDir()}
		if _, err := migrator.Reset(); err == nil {
			t.Fatal("expected error reading a migration file that no longer exists on disk, got nil")
		}
	})
}

func TestMigrator_Status_ErrorPaths(t *testing.T) {
	t.Run("GetAppliedMigrations error", func(t *testing.T) {
		driver := &errDriver{failApplied: errors.New("boom")}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: t.TempDir()}
		if _, err := migrator.Status(); err == nil {
			t.Fatal("expected error from GetAppliedMigrations, got nil")
		}
	})

	t.Run("ReadDir error", func(t *testing.T) {
		driver := &db.MockDriver{}
		migrator := &migrate.Migrator{Driver: driver, Registry: newSQLiteRegistry(t), Dir: "/nonexistent/path/for/mig/tests"}
		if _, err := migrator.Status(); err == nil {
			t.Fatal("expected error from os.ReadDir on a nonexistent dir, got nil")
		}
	})
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

func TestMigrator_Reset(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migrations")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	names := []string{"2026_01_01_000000_first.sql", "2026_01_02_000000_second.sql"}
	for _, n := range names {
		content := "-- +migrate Up\nUP\n-- +migrate Down\nDOWN"
		if err := os.WriteFile(filepath.Join(tmpDir, n), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
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
		t.Fatalf("expected 2 applied migrations before reset, got %d", len(driver.AppliedMigrations))
	}

	rolledBack, err := migrator.Reset()
	if err != nil {
		t.Fatal(err)
	}
	if len(rolledBack) != 2 {
		t.Fatalf("expected 2 rolled back migrations, got %d", len(rolledBack))
	}
	// Reset rolls back most-recently-applied first.
	if rolledBack[0] != names[1] || rolledBack[1] != names[0] {
		t.Errorf("expected reverse order [%s, %s], got %v", names[1], names[0], rolledBack)
	}
	if len(driver.AppliedMigrations) != 0 {
		t.Error("Reset did not clear all applied migrations")
	}
}

func TestMigrator_Reset_NoAppliedMigrations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "migrations")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	driver := &db.MockDriver{}
	migrator := &migrate.Migrator{
		Driver:   driver,
		Registry: newSQLiteRegistry(t),
		Dir:      tmpDir,
	}

	rolledBack, err := migrator.Reset()
	if err != nil {
		t.Fatal(err)
	}
	if len(rolledBack) != 0 {
		t.Errorf("expected no migrations rolled back, got %v", rolledBack)
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
