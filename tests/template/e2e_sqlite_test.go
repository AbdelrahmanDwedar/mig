package template_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/AbdelrahmanDwedar/mig/internal/config"
	"github.com/AbdelrahmanDwedar/mig/internal/db"
	"github.com/AbdelrahmanDwedar/mig/internal/migrate"
	"github.com/AbdelrahmanDwedar/mig/internal/parser"
	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
	"github.com/AbdelrahmanDwedar/mig/internal/template"
)

// buildScaffoldedMigrations generates a create_table -> add_column ->
// add_index chain via the real template renderer (for the given format)
// and writes them into dir, returning the applied-order filenames.
func buildScaffoldedMigrations(t *testing.T, dir, format string, dialect sqlgen.Dialect) []string {
	t.Helper()

	steps := []struct {
		suffix string
		kind   string
		flags  template.Flags
	}{
		{"0001_create_widgets", "create_table", template.Flags{
			Table: "widgets", Columns: "id:integer:pk:auto,name:string(100)",
		}},
		{"0002_add_description", "add_column", template.Flags{
			Table: "widgets", Columns: "description:text:null",
		}},
		{"0003_index_name", "add_index", template.Flags{
			Table: "widgets", IndexColumns: "name", IndexName: "idx_widgets_name",
		}},
	}

	ext := "sql"
	if format == "json" {
		ext = "json"
	}

	var names []string
	for _, s := range steps {
		op, err := template.BuildOp(s.kind, s.flags)
		if err != nil {
			t.Fatalf("BuildOp(%s): %v", s.kind, err)
		}

		var content string
		if format == "json" {
			content, err = template.RenderJSON(op)
		} else {
			content, err = template.RenderSQL(dialect, op)
		}
		if err != nil {
			t.Fatalf("Render(%s): %v", s.kind, err)
		}

		name := s.suffix + "." + ext
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

func newSQLiteMigrator(t *testing.T, dbPath, dir string) *migrate.Migrator {
	t.Helper()

	dialect, err := sqlgen.New("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	reg := parser.NewRegistry(dialect)

	driver, err := db.NewDriver(&config.DatabaseConfig{Driver: "sqlite", DBName: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { driver.Close() })

	return &migrate.Migrator{Driver: driver, Registry: reg, Dir: dir}
}

func tableColumnNames(t *testing.T, dbPath, table string) []string {
	t.Helper()
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	rows, err := conn.Query("SELECT name FROM pragma_table_info(?)", table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, name)
	}
	return cols
}

func indexExists(t *testing.T, dbPath, table, index string) bool {
	t.Helper()
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	rows, err := conn.Query("SELECT name FROM pragma_index_list(?)", table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if name == index {
			return true
		}
	}
	return false
}

func tableExists(t *testing.T, dbPath, table string) bool {
	t.Helper()
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var name string
	err = conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
	return err == nil
}

// TestE2E_SQLite_ScaffoldedMigrationsApplyAndReverse proves that a
// create_table -> add_column -> add_index chain of migrations scaffolded
// entirely via the template renderer (not hand-written) actually executes
// successfully through the real Migrator and reverses correctly — the one
// thing the golden-comparison unit tests in template_test.go can't catch.
func TestE2E_SQLite_ScaffoldedMigrationsApplyAndReverse(t *testing.T) {
	for _, format := range []string{"sql", "json"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "test.db")
			migrationsDir := filepath.Join(dir, "migrations")
			if err := os.MkdirAll(migrationsDir, 0755); err != nil {
				t.Fatal(err)
			}

			dialect, err := sqlgen.New("sqlite")
			if err != nil {
				t.Fatal(err)
			}
			buildScaffoldedMigrations(t, migrationsDir, format, dialect)

			m := newSQLiteMigrator(t, dbPath, migrationsDir)

			applied, err := m.Migrate()
			if err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			if len(applied) != 3 {
				t.Fatalf("expected 3 migrations applied, got %d: %v", len(applied), applied)
			}

			cols := tableColumnNames(t, dbPath, "widgets")
			wantCols := map[string]bool{"id": true, "name": true, "description": true}
			if len(cols) != len(wantCols) {
				t.Fatalf("expected columns %v, got %v", wantCols, cols)
			}
			for _, c := range cols {
				if !wantCols[c] {
					t.Errorf("unexpected column %q in widgets: %v", c, cols)
				}
			}
			if !indexExists(t, dbPath, "widgets", "idx_widgets_name") {
				t.Fatal("expected idx_widgets_name to exist after Migrate")
			}

			rolledBack, err := m.Rollback(3, "")
			if err != nil {
				t.Fatalf("Rollback: %v", err)
			}
			if len(rolledBack) != 3 {
				t.Fatalf("expected 3 migrations rolled back, got %d: %v", len(rolledBack), rolledBack)
			}

			if tableExists(t, dbPath, "widgets") {
				t.Fatal("expected widgets table to be gone after full rollback")
			}
		})
	}
}
