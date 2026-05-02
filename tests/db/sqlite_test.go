package db_test

import (
	"os"
	"testing"

	"github.com/mig-tool/mig/internal/config"
	"github.com/mig-tool/mig/internal/db"
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
