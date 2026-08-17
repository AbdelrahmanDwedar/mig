package db_test

import (
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/config"
	"github.com/AbdelrahmanDwedar/mig/internal/db"
)

// mysqlDriverForTest returns a connected MySQLDriver. sql.Open is lazy for
// this driver (it doesn't dial), so Connect/Close can be exercised without a
// live server; the DB-hitting methods below are expected to fail since
// nothing is actually listening on the configured port.
func mysqlDriverForTest(t *testing.T) db.Driver {
	t.Helper()
	d, err := db.NewDriver(&config.DatabaseConfig{
		Driver:   "mysql",
		Host:     "127.0.0.1",
		Port:     54329,
		User:     "user",
		Password: "password",
		DBName:   "mig_test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Connect(); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestMySQLDriver_ConnectClose(t *testing.T) {
	d := mysqlDriverForTest(t)
	if err := d.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
}

func TestMySQLDriver_UnreachableServer(t *testing.T) {
	d := mysqlDriverForTest(t)
	defer d.Close()

	if err := d.EnsureMigrationsTable(); err == nil {
		t.Error("expected error from EnsureMigrationsTable with no server listening")
	}
	if _, err := d.GetAppliedMigrations(); err == nil {
		t.Error("expected error from GetAppliedMigrations with no server listening")
	}
	if err := d.ApplyMigration("m.sql", "SELECT 1"); err == nil {
		t.Error("expected error from ApplyMigration with no server listening")
	}
	if err := d.RollbackMigration("m.sql", "SELECT 1"); err == nil {
		t.Error("expected error from RollbackMigration with no server listening")
	}
}
