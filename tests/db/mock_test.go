package db_test

import (
	"database/sql"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/db"
	_ "github.com/mattn/go-sqlite3"
)

func TestMockDriver_Conn_Nil(t *testing.T) {
	m := &db.MockDriver{}
	if m.Conn() != nil {
		t.Error("expected Conn() to return nil when DB was never set")
	}
}

func TestMockDriver_Conn_Set(t *testing.T) {
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	m := &db.MockDriver{DB: conn}
	if m.Conn() != conn {
		t.Error("expected Conn() to return the exact *sql.DB stored on the mock")
	}
}

func TestMockDriver_EnsureMigrationsTable(t *testing.T) {
	m := &db.MockDriver{}
	if err := m.EnsureMigrationsTable(); err != nil {
		t.Errorf("expected EnsureMigrationsTable() to always succeed, got %v", err)
	}
}

func TestMockDriver_GetAppliedMigrations(t *testing.T) {
	m := &db.MockDriver{AppliedMigrations: []string{"001_init.sql", "002_add_widgets.sql"}}
	got, err := m.GetAppliedMigrations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "001_init.sql" || got[1] != "002_add_widgets.sql" {
		t.Errorf("expected the stored AppliedMigrations slice verbatim, got %v", got)
	}
}

func TestMockDriver_ApplyMigration(t *testing.T) {
	m := &db.MockDriver{}
	if err := m.ApplyMigration("001_init.sql", "CREATE TABLE widgets(id INTEGER)"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.AppliedMigrations) != 1 || m.AppliedMigrations[0] != "001_init.sql" {
		t.Errorf("expected ApplyMigration to append the name, got %v", m.AppliedMigrations)
	}
}

func TestMockDriver_RollbackMigration_Removes(t *testing.T) {
	m := &db.MockDriver{AppliedMigrations: []string{"001_init.sql", "002_add_widgets.sql"}}
	if err := m.RollbackMigration("001_init.sql", "DROP TABLE widgets"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.AppliedMigrations) != 1 || m.AppliedMigrations[0] != "002_add_widgets.sql" {
		t.Errorf("expected only 002_add_widgets.sql to remain, got %v", m.AppliedMigrations)
	}
}
