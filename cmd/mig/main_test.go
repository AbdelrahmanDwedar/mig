package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/config"
	"github.com/spf13/cobra"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func formatCmd(t *testing.T, changed bool, value string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "create"}
	cmd.Flags().String("format", "sql", "")
	if changed {
		if err := cmd.Flags().Set("format", value); err != nil {
			t.Fatal(err)
		}
	}
	return cmd
}

func TestResolveFormat(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		flagSet   bool
		cfg       *config.Config
		want      string
		wantErr   bool
	}{
		{name: "default sql, no cfg", want: "sql"},
		{
			name: "cfg overrides default",
			cfg:  &config.Config{Migrations: config.MigrationsConfig{Parser: "json"}},
			want: "json",
		},
		{
			name:      "explicit flag beats cfg",
			flagValue: "json",
			flagSet:   true,
			cfg:       &config.Config{Migrations: config.MigrationsConfig{Parser: "sql"}},
			want:      "json",
		},
		{
			name:      "explicit flag accepts yaml",
			flagValue: "yaml",
			flagSet:   true,
			want:      "yaml",
		},
		{
			name:      "invalid format errors",
			flagValue: "xml",
			flagSet:   true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := formatCmd(t, tt.flagSet, tt.flagValue)
			got, err := resolveFormat(cmd, tt.flagValue, tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunSetup_SQLite(t *testing.T) {
	t.Chdir(t.TempDir())

	created, err := runSetup("sqlite", "test.db", "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Error("expected created=true on first run")
	}
	if _, err := os.Stat("mig.yml"); err != nil {
		t.Errorf("mig.yml not created: %v", err)
	}
	if _, err := os.Stat("migrations"); err != nil {
		t.Errorf("migrations dir not created: %v", err)
	}

	// Second run should be a no-op since mig.yml already exists.
	created, err = runSetup("sqlite", "test.db", "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("expected created=false when mig.yml already exists")
	}
}

func TestRunSetup_NonSQLiteDriver(t *testing.T) {
	t.Chdir(t.TempDir())

	created, err := runSetup("postgresql", "mydb", "migrations")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Error("expected created=true")
	}
	data, err := os.ReadFile("mig.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "driver: postgresql") || !strings.Contains(string(data), "host: localhost") {
		t.Errorf("mig.yml missing expected postgresql template content:\n%s", data)
	}
}

func TestRunSetup_MkdirError(t *testing.T) {
	t.Chdir(t.TempDir())

	// Create a *file* named "migrations" so os.MkdirAll("migrations", ...) fails.
	if err := os.WriteFile("migrations", []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := runSetup("sqlite", "test.db", "migrations"); err == nil {
		t.Fatal("expected error when migrations dir path is blocked by a file, got nil")
	}
}

func TestCreateMigration(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := runSetup("sqlite", "test.db", "migrations"); err != nil {
		t.Fatal(err)
	}

	sqlFile, err := createMigration("add_widgets", "sql")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(sqlFile) != ".sql" {
		t.Errorf("expected .sql file, got %s", sqlFile)
	}

	jsonFile, err := createMigration("add_gadgets", "json")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(jsonFile) != ".json" {
		t.Errorf("expected .json file, got %s", jsonFile)
	}
	content, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"up"`) {
		t.Errorf("expected JSON boilerplate, got %s", content)
	}
}

func TestCreateMigration_WriteError(t *testing.T) {
	t.Chdir(t.TempDir())
	// No mig.yml/migrations dir exists, so the write should fail.
	if _, err := createMigration("add_widgets", "sql"); err == nil {
		t.Fatal("expected error writing migration into a nonexistent directory, got nil")
	}
}

func TestNewRegistry(t *testing.T) {
	if _, err := newRegistry(&config.Config{Database: config.DatabaseConfig{Driver: "sqlite"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := newRegistry(&config.Config{Database: config.DatabaseConfig{Driver: "oracle"}}); err == nil {
		t.Fatal("expected error for unsupported driver, got nil")
	}
}

func TestNewMigrator(t *testing.T) {
	t.Chdir(t.TempDir())

	if _, err := newMigrator(); err == nil {
		t.Fatal("expected error when mig.yml is missing, got nil")
	}

	if _, err := runSetup("sqlite", "test.db", "migrations"); err != nil {
		t.Fatal(err)
	}
	m, err := newMigrator()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Driver.Close()
	if m.Dir != "migrations" {
		t.Errorf("expected dir migrations, got %s", m.Dir)
	}
}

func TestPrintResult(t *testing.T) {
	t.Run("json success", func(t *testing.T) {
		jsonOutput = true
		defer func() { jsonOutput = false }()

		out := captureStdout(t, func() {
			if err := printResult(map[string]any{"x": 1}, nil, nil); err != nil {
				t.Fatal(err)
			}
		})
		var r Result
		if err := json.Unmarshal([]byte(out), &r); err != nil {
			t.Fatalf("output is not valid JSON: %v (%s)", err, out)
		}
		if !r.Success {
			t.Error("expected Success=true")
		}
	})

	t.Run("json error", func(t *testing.T) {
		jsonOutput = true
		defer func() { jsonOutput = false }()

		wantErr := os.ErrNotExist
		out := captureStdout(t, func() {
			_ = printResult(nil, wantErr, nil)
		})
		var r Result
		if err := json.Unmarshal([]byte(out), &r); err != nil {
			t.Fatalf("output is not valid JSON: %v (%s)", err, out)
		}
		if r.Success || r.Error == "" {
			t.Errorf("expected Success=false with an error message, got %+v", r)
		}
	})

	t.Run("plain success calls plainFn", func(t *testing.T) {
		called := false
		if err := printResult(nil, nil, func() { called = true }); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Error("expected plainFn to be called on success")
		}
	})

	t.Run("plain error skips plainFn", func(t *testing.T) {
		called := false
		wantErr := os.ErrNotExist
		err := printResult(nil, wantErr, func() { called = true })
		if err != wantErr {
			t.Errorf("expected error to propagate unchanged, got %v", err)
		}
		if called {
			t.Error("plainFn should not be called on error")
		}
	})
}

// runCLI builds a fresh root command (resetting all package-level flag vars)
// and executes it with args inside the current working directory.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var stdout string
	var err error
	stdout = captureStdout(t, func() {
		cmd := NewRootCmd()
		cmd.SetArgs(args)
		err = cmd.Execute()
	})
	return stdout, err
}

func TestCLI_FullLifecycle(t *testing.T) {
	t.Chdir(t.TempDir())

	// setup
	out, err := runCLI(t, "setup", "--driver", "sqlite", "--dbname", "test.db", "--dir", "migrations", "--json")
	if err != nil {
		t.Fatalf("setup: %v (%s)", err, out)
	}
	var setupResult Result
	if err := json.Unmarshal([]byte(out), &setupResult); err != nil {
		t.Fatalf("setup output not JSON: %v (%s)", err, out)
	}
	if !setupResult.Success {
		t.Fatalf("setup did not succeed: %+v", setupResult)
	}

	// create (json format)
	out, err = runCLI(t, "create", "add_widgets", "--format", "json", "--json")
	if err != nil {
		t.Fatalf("create: %v (%s)", err, out)
	}
	var createResult Result
	if err := json.Unmarshal([]byte(out), &createResult); err != nil {
		t.Fatalf("create output not JSON: %v (%s)", err, out)
	}
	data, _ := createResult.Data.(map[string]any)
	filename, _ := data["file"].(string)
	if filename == "" || filepath.Ext(filename) != ".json" {
		t.Fatalf("expected a .json migration file, got %+v", createResult.Data)
	}

	// fill in the JSON migration with an actual create_table op.
	migrationContent := `{
		"up": [{"op": "create_table", "table": "widgets", "columns": [{"name": "id", "type": "integer"}]}],
		"down": [{"op": "drop_table", "table": "widgets", "if_exists": true}]
	}`
	if err := os.WriteFile(filename, []byte(migrationContent), 0644); err != nil {
		t.Fatal(err)
	}

	// create (default sql format, no --format flag)
	out, err = runCLI(t, "create", "add_gadgets", "--json")
	if err != nil {
		t.Fatalf("create (sql): %v (%s)", err, out)
	}

	// migrate
	out, err = runCLI(t, "migrate", "--json")
	if err != nil {
		t.Fatalf("migrate: %v (%s)", err, out)
	}
	var migrateResult Result
	if err := json.Unmarshal([]byte(out), &migrateResult); err != nil {
		t.Fatalf("migrate output not JSON: %v (%s)", err, out)
	}
	if !migrateResult.Success {
		t.Fatalf("migrate did not succeed: %+v", migrateResult)
	}

	// status
	out, err = runCLI(t, "status", "--json")
	if err != nil {
		t.Fatalf("status: %v (%s)", err, out)
	}

	// status (plain output path)
	out, err = runCLI(t, "status")
	if err != nil {
		t.Fatalf("status (plain): %v (%s)", err, out)
	}
	if !strings.Contains(out, "widgets") && !strings.Contains(out, "add_widgets") {
		t.Errorf("expected plain status output to mention the migration, got %s", out)
	}

	// rollback --steps 1
	out, err = runCLI(t, "rollback", "--steps", "1", "--json")
	if err != nil {
		t.Fatalf("rollback: %v (%s)", err, out)
	}

	// rollback --steps and --migration together should error via PreRunE
	_, err = runCLI(t, "rollback", "--steps", "1", "--migration", "add_widgets")
	if err == nil {
		t.Fatal("expected error when --steps and --migration are combined")
	}

	// fresh (reset + migrate)
	out, err = runCLI(t, "fresh", "--json")
	if err != nil {
		t.Fatalf("fresh: %v (%s)", err, out)
	}

	// refresh (alias of fresh)
	out, err = runCLI(t, "refresh", "--json")
	if err != nil {
		t.Fatalf("refresh: %v (%s)", err, out)
	}

	// reset
	out, err = runCLI(t, "reset", "--json")
	if err != nil {
		t.Fatalf("reset: %v (%s)", err, out)
	}
}

func TestCLI_MigrateWithoutSetup(t *testing.T) {
	t.Chdir(t.TempDir())

	out, err := runCLI(t, "migrate", "--json")
	if err == nil {
		t.Fatal("expected an error when mig.yml is missing")
	}
	var r Result
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("output not JSON: %v (%s)", err, out)
	}
	if r.Success {
		t.Error("expected Success=false when mig.yml is missing")
	}
}

func TestCLI_CreateTemplate_SQL(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := runSetup("sqlite", "test.db", "migrations"); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, "create", "add_users", "--template", "create_table",
		"--table", "users", "--columns", "id:bigint:pk:auto,email:string(255):unique", "--json")
	if err != nil {
		t.Fatalf("create --template: %v (%s)", err, out)
	}
	var result Result
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output not JSON: %v (%s)", err, out)
	}
	if !result.Success {
		t.Fatalf("create --template did not succeed: %+v", result)
	}
	data, _ := result.Data.(map[string]any)
	filename, _ := data["file"].(string)
	if filename == "" || filepath.Ext(filename) != ".sql" {
		t.Fatalf("expected a .sql migration file, got %+v", result.Data)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "CREATE TABLE") || !strings.Contains(string(content), "users") {
		t.Errorf("expected dialect-rendered CREATE TABLE, got:\n%s", content)
	}
	if !strings.Contains(string(content), "-- +migrate Up") || !strings.Contains(string(content), "-- +migrate Down") {
		t.Errorf("expected migrate markers, got:\n%s", content)
	}
	if !strings.Contains(string(content), "DROP TABLE") {
		t.Errorf("expected mechanically-derived down (DROP TABLE), got:\n%s", content)
	}
}

func TestCLI_CreateTemplate_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := runSetup("sqlite", "test.db", "migrations"); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, "create", "add_users", "--format", "json", "--template", "create_table",
		"--table", "users", "--columns", "id:bigint:pk:auto,email:string(255):unique", "--json")
	if err != nil {
		t.Fatalf("create --template: %v (%s)", err, out)
	}
	var result Result
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output not JSON: %v (%s)", err, out)
	}
	data, _ := result.Data.(map[string]any)
	filename, _ := data["file"].(string)
	if filename == "" || filepath.Ext(filename) != ".json" {
		t.Fatalf("expected a .json migration file, got %+v", result.Data)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"create_table"`) || !strings.Contains(string(content), `"drop_table"`) {
		t.Errorf("expected create_table/drop_table op envelope, got:\n%s", content)
	}
}

func TestCLI_CreateTemplate_UnknownTemplate(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := runSetup("sqlite", "test.db", "migrations"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "create", "bogus", "--template", "not_a_real_op", "--table", "users"); err == nil {
		t.Fatal("expected error for unknown --template value")
	}
}

func TestCLI_CreateTemplate_MissingRequiredFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"create_table missing --columns", []string{"create", "n", "--template", "create_table", "--table", "users"}},
		{"add_column missing --columns", []string{"create", "n", "--template", "add_column", "--table", "users"}},
		{"drop_column missing --column", []string{"create", "n", "--template", "drop_column", "--table", "users"}},
		{"rename_table missing --to", []string{"create", "n", "--template", "rename_table", "--table", "users"}},
		{"add_index missing --index", []string{"create", "n", "--template", "add_index", "--table", "users"}},
		{"drop_index missing --index-name", []string{"create", "n", "--template", "drop_index", "--table", "users"}},
		{"add_foreign_key missing --fk-references", []string{"create", "n", "--template", "add_foreign_key", "--table", "posts", "--fk-column", "user_id"}},
		{"drop_foreign_key missing --fk-name", []string{"create", "n", "--template", "drop_foreign_key", "--table", "posts"}},
		{"missing --table entirely", []string{"create", "n", "--template", "create_table", "--columns", "id:integer"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if _, err := runSetup("sqlite", "test.db", "migrations"); err != nil {
				t.Fatal(err)
			}
			if _, err := runCLI(t, c.args...); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestCLI_CreateTemplate_RequiresConfigForSQLFormat(t *testing.T) {
	t.Chdir(t.TempDir())
	// No mig.yml, so there's no driver to resolve a dialect from.
	if _, err := runCLI(t, "create", "add_users", "--template", "create_table",
		"--table", "users", "--columns", "id:integer"); err == nil {
		t.Fatal("expected error resolving dialect without mig.yml")
	}
}

func TestCLI_Create_NoTemplate_Unchanged(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := runSetup("sqlite", "test.db", "migrations"); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, "create", "add_widgets", "--json")
	if err != nil {
		t.Fatalf("create: %v (%s)", err, out)
	}
	var result Result
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output not JSON: %v (%s)", err, out)
	}
	data, _ := result.Data.(map[string]any)
	filename, _ := data["file"].(string)

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != migrationBoilerplate {
		t.Errorf("expected byte-identical boilerplate when --template is unset, got:\n%s", content)
	}
}
