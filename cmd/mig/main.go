package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AbdelrahmanDwedar/mig/internal/config"
	"github.com/AbdelrahmanDwedar/mig/internal/db"
	"github.com/AbdelrahmanDwedar/mig/internal/migrate"
	"github.com/AbdelrahmanDwedar/mig/internal/parser"
	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
	"github.com/AbdelrahmanDwedar/mig/internal/template"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const migrationBoilerplate = `-- +migrate Up
-- SQL queries for UP migration here

-- +migrate Down
-- SQL queries for DOWN migration here
`

var jsonOutput bool

// Result is the envelope printed to stdout when --json is passed.
type Result struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// printResult prints either the JSON envelope or, on success, invokes plainFn
// for human-readable output. It returns err unchanged so callers can still
// propagate it for the exit code.
func printResult(data any, err error, plainFn func()) error {
	if jsonOutput {
		r := Result{Success: err == nil, Data: data}
		if err != nil {
			r.Error = err.Error()
		}
		b, marshalErr := json.Marshal(r)
		if marshalErr != nil {
			return marshalErr
		}
		fmt.Println(string(b))
		return err
	}
	if err != nil {
		return err
	}
	if plainFn != nil {
		plainFn()
	}
	return nil
}

const jsonMigrationBoilerplate = `{
  "up": [],
  "down": []
}
`

const yamlMigrationBoilerplate = `down: []
up: []
`

func runSetup(driver, dbName, dir string) (created bool, err error) {
	if _, err := os.Stat("mig.yml"); err == nil {
		if !jsonOutput {
			fmt.Println("Configuration file 'mig.yml' already exists. Skipping initialization.")
		}
		return false, nil
	}

	if driver == "" {
		promptDriver := promptui.Select{
			Label: "Select Database Driver",
			Items: []string{"postgresql", "mysql", "sqlite"},
		}
		_, selected, err := promptDriver.Run()
		if err != nil {
			return false, err
		}
		driver = selected
	}

	if driver == "sqlite" && dbName == "" {
		promptDB := promptui.Prompt{
			Label:   "SQLite Database Filename",
			Default: "database.db",
		}
		dbName, _ = promptDB.Run()
	} else if dbName == "" {
		dbName = "mydatabase"
	}

	if dir == "" {
		promptDir := promptui.Prompt{
			Label:   "Migration Directory",
			Default: "migrations",
		}
		dir, _ = promptDir.Run()
	}

	var cfg string
	if driver == "sqlite" {
		cfg = fmt.Sprintf(`database:
  driver: sqlite
  # host: localhost
  # port: 5432
  # user: user
  # password: password
  dbname: %s
migrations:
  parser: sql # or json/yaml -- sets the default format for 'mig create'
  dir: %s
`, dbName, dir)
	} else {
		cfg = fmt.Sprintf(`database:
  driver: %s
  host: localhost
  port: 5432
  user: user
  password: password
  dbname: %s
migrations:
  parser: sql # or json/yaml -- sets the default format for 'mig create'
  dir: %s
`, driver, dbName, dir)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create migrations directory: %w", err)
	}

	if err := os.WriteFile("mig.yml", []byte(cfg), 0644); err != nil {
		return false, fmt.Errorf("failed to create mig.yml: %w", err)
	}

	if !jsonOutput {
		fmt.Println("Project initialized: created " + dir + "/ and mig.yml")
	}
	return true, nil
}

// resolveFormat applies the format precedence for `mig create`: an explicit
// --format flag wins, then mig.yml's migrations.parser, then "sql".
func resolveFormat(cmd *cobra.Command, formatFlag string, cfg *config.Config) (string, error) {
	format := "sql"
	if cfg != nil && cfg.Migrations.Parser != "" {
		format = cfg.Migrations.Parser
	}
	if cmd.Flag("format").Changed {
		format = formatFlag
	}

	switch format {
	case "sql", "json", "yaml":
		return format, nil
	default:
		return "", fmt.Errorf("unsupported format: %q (must be \"sql\", \"json\", or \"yaml\")", format)
	}
}

func createMigration(name, format string) (string, error) {
	cfg, err := config.LoadConfig("mig.yml")
	dir := "migrations"
	if err == nil && cfg.Migrations.Dir != "" {
		dir = cfg.Migrations.Dir
	}

	timestamp := time.Now().Format("2006_01_02_150405")

	var extension, boilerplate string
	switch format {
	case "json":
		extension, boilerplate = "json", jsonMigrationBoilerplate
	case "yaml":
		extension, boilerplate = "yaml", yamlMigrationBoilerplate
	default:
		extension, boilerplate = "sql", migrationBoilerplate
	}

	filename := fmt.Sprintf("%s/%s_%s.%s", dir, timestamp, name, extension)

	if err := os.WriteFile(filename, []byte(boilerplate), 0644); err != nil {
		return "", fmt.Errorf("failed to create migration file: %w", err)
	}
	if !jsonOutput {
		fmt.Printf("Created migration: %s\n", filename)
	}
	return filename, nil
}

// createMigrationFromTemplate scaffolds a migration from a resolved
// template Op instead of the empty boilerplate, rendering it via the same
// dir/timestamp/extension conventions as createMigration. dialect may be
// nil when format is "json" or "yaml" (dialect resolution for those
// structured formats happens later, at migrate-time).
func createMigrationFromTemplate(name, format string, op template.Op, dialect sqlgen.Dialect) (string, error) {
	cfg, err := config.LoadConfig("mig.yml")
	dir := "migrations"
	if err == nil && cfg.Migrations.Dir != "" {
		dir = cfg.Migrations.Dir
	}

	var content, extension string
	switch format {
	case "json":
		extension = "json"
		content, err = template.RenderJSON(op)
	case "yaml":
		extension = "yaml"
		content, err = template.RenderYAML(op)
	default:
		extension = "sql"
		content, err = template.RenderSQL(dialect, op)
	}
	if err != nil {
		return "", err
	}

	timestamp := time.Now().Format("2006_01_02_150405")
	filename := fmt.Sprintf("%s/%s_%s.%s", dir, timestamp, name, extension)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to create migration file: %w", err)
	}
	if !jsonOutput {
		fmt.Printf("Created migration: %s\n", filename)
	}
	return filename, nil
}

func newRegistry(cfg *config.Config) (*parser.Registry, error) {
	dialect, err := sqlgen.New(cfg.Database.Driver)
	if err != nil {
		return nil, err
	}
	return parser.NewRegistry(dialect), nil
}

// newMigrator loads mig.yml, constructs and connects a driver, and returns a
// ready-to-use Migrator. The caller is responsible for closing the driver.
func newMigrator() (*migrate.Migrator, error) {
	cfg, err := config.LoadConfig("mig.yml")
	if err != nil {
		return nil, err
	}
	driver, err := db.NewDriver(&cfg.Database)
	if err != nil {
		return nil, err
	}
	reg, err := newRegistry(cfg)
	if err != nil {
		return nil, err
	}
	if err := driver.Connect(); err != nil {
		return nil, err
	}

	dir := "migrations"
	if cfg.Migrations.Dir != "" {
		dir = cfg.Migrations.Dir
	}

	return &migrate.Migrator{Driver: driver, Registry: reg, Dir: dir}, nil
}

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{Use: "mig", SilenceUsage: true, SilenceErrors: true}
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output machine-readable JSON")

	var driverFlag, dbNameFlag, dirFlag string
	var setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Initialize the migration project",
		RunE: func(cmd *cobra.Command, args []string) error {
			created, err := runSetup(driverFlag, dbNameFlag, dirFlag)
			data := map[string]any{
				"driver":  driverFlag,
				"dbname":  dbNameFlag,
				"dir":     dirFlag,
				"created": created,
			}
			return printResult(data, err, nil)
		},
	}
	setupCmd.Flags().StringVar(&driverFlag, "driver", "", "Database driver (postgresql, mysql, sqlite)")
	setupCmd.Flags().StringVar(&dbNameFlag, "dbname", "", "Database name or SQLite file")
	setupCmd.Flags().StringVar(&dirFlag, "dir", "", "Migration directory")

	var formatFlag, templateFlag, tableFlag, columnsFlag, columnFlag, toFlag string
	var indexColsFlag, indexNameFlag, fkReferencesFlag, fkOnDeleteFlag, fkOnUpdateFlag, fkNameFlag string
	var fkColumnsFlag []string
	var indexUniqueFlag, ifNotExistsFlag, ifExistsFlag bool
	var createCmd = &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new migration file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadConfig("mig.yml")
			format, err := resolveFormat(cmd, formatFlag, cfg)
			if err != nil {
				return err
			}

			if templateFlag == "" {
				filename, err := createMigration(args[0], format)
				return printResult(map[string]any{"file": filename}, err, nil)
			}

			op, err := template.BuildOp(templateFlag, template.Flags{
				Table:        tableFlag,
				Columns:      columnsFlag,
				Column:       columnFlag,
				To:           toFlag,
				IndexColumns: indexColsFlag,
				IndexName:    indexNameFlag,
				IndexUnique:  indexUniqueFlag,
				FKColumns:    fkColumnsFlag,
				FKReferences: fkReferencesFlag,
				FKOnDelete:   fkOnDeleteFlag,
				FKOnUpdate:   fkOnUpdateFlag,
				FKName:       fkNameFlag,
				IfNotExists:  ifNotExistsFlag,
				IfExists:     ifExistsFlag,
			})
			if err != nil {
				return err
			}

			var dialect sqlgen.Dialect
			if format == "sql" {
				if cfg == nil {
					return fmt.Errorf("--template with --format sql requires a mig.yml to resolve the database dialect (run 'mig setup' first, or use --format json/yaml)")
				}
				dialect, err = sqlgen.New(cfg.Database.Driver)
				if err != nil {
					return err
				}
			}

			filename, err := createMigrationFromTemplate(args[0], format, op, dialect)
			return printResult(map[string]any{"file": filename}, err, nil)
		},
	}
	createCmd.Flags().StringVar(&formatFlag, "format", "sql", "Migration file format (sql, json, yaml)")
	createCmd.Flags().StringVar(&templateFlag, "template", "", "Scaffold a common op: "+strings.Join(template.SupportedTemplates, "|"))
	createCmd.Flags().StringVar(&tableFlag, "table", "", "Target table name (all templates)")
	createCmd.Flags().StringVar(&columnsFlag, "columns", "", `Column spec, e.g. "id:bigint:pk:auto,email:string(255):unique" (create_table, add_column)`)
	createCmd.Flags().StringVar(&columnFlag, "column", "", "Column name (drop_column, rename_column)")
	createCmd.Flags().StringVar(&toFlag, "to", "", "New name (rename_table, rename_column)")
	createCmd.Flags().StringVar(&indexColsFlag, "index", "", "Comma-separated columns (add_index)")
	createCmd.Flags().StringVar(&indexNameFlag, "index-name", "", "Index name (add_index, drop_index)")
	createCmd.Flags().BoolVar(&indexUniqueFlag, "index-unique", false, "Index is UNIQUE (add_index)")
	createCmd.Flags().StringArrayVar(&fkColumnsFlag, "fk-column", nil, "Foreign key source column, repeatable for composite keys (add_foreign_key)")
	createCmd.Flags().StringVar(&fkReferencesFlag, "fk-references", "", `Referenced table(cols), e.g. "users(id)" (add_foreign_key)`)
	createCmd.Flags().StringVar(&fkOnDeleteFlag, "fk-on-delete", "", "ON DELETE action (add_foreign_key)")
	createCmd.Flags().StringVar(&fkOnUpdateFlag, "fk-on-update", "", "ON UPDATE action (add_foreign_key)")
	createCmd.Flags().StringVar(&fkNameFlag, "fk-name", "", "Foreign key constraint name (add_foreign_key, drop_foreign_key)")
	createCmd.Flags().BoolVar(&ifNotExistsFlag, "if-not-exists", false, "Add IF NOT EXISTS (create_table)")
	createCmd.Flags().BoolVar(&ifExistsFlag, "if-exists", false, "Add IF EXISTS (drop_table)")

	var migrateCmd = &cobra.Command{
		Use:   "migrate",
		Short: "Run pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrator, err := newMigrator()
			if err != nil {
				return printResult(nil, err, nil)
			}
			defer migrator.Driver.Close()

			applied, err := migrator.Migrate()
			return printResult(map[string]any{"applied": applied}, err, func() {
				for _, name := range applied {
					fmt.Printf("Applying migration: %s\n", name)
				}
			})
		},
	}

	var steps int
	var migrationPath string

	var rollbackCmd = &cobra.Command{
		Use:   "rollback",
		Short: "Rollback migrations",
		Long: `Rollback previously applied migrations.
You can either specify a number of steps to rollback with --steps (-s),
or target a specific migration file with --migration (-m).
Note: These flags are mutually exclusive.`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			stepsFlag := cmd.Flag("steps").Changed
			migrationFlag := cmd.Flag("migration").Changed
			if stepsFlag && migrationFlag {
				return fmt.Errorf("cannot use --steps and --migration flags together")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			migrator, err := newMigrator()
			if err != nil {
				return printResult(nil, err, nil)
			}
			defer migrator.Driver.Close()

			rolledBack, err := migrator.Rollback(steps, migrationPath)
			return printResult(map[string]any{"rolled_back": rolledBack}, err, func() {
				for _, name := range rolledBack {
					fmt.Printf("Rolling back migration: %s\n", name)
				}
			})
		},
	}
	rollbackCmd.Flags().IntVarP(&steps, "steps", "s", 1, "Number of steps to rollback")
	rollbackCmd.Flags().StringVarP(&migrationPath, "migration", "m", "", "Path to specific migration to rollback")

	var resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Rollback all migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrator, err := newMigrator()
			if err != nil {
				return printResult(nil, err, nil)
			}
			defer migrator.Driver.Close()

			rolledBack, err := migrator.Reset()
			return printResult(map[string]any{"rolled_back": rolledBack}, err, func() {
				for _, name := range rolledBack {
					fmt.Printf("Rolling back migration: %s\n", name)
				}
			})
		},
	}

	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Display migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrator, err := newMigrator()
			if err != nil {
				return printResult(nil, err, nil)
			}
			defer migrator.Driver.Close()

			status, err := migrator.Status()
			return printResult(status, err, func() {
				for _, s := range status {
					fmt.Printf("%s: %s\n", s["name"], s["status"])
				}
			})
		},
	}

	var freshCmd = &cobra.Command{
		Use:   "fresh",
		Short: "Reset and re-run all migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrator, err := newMigrator()
			if err != nil {
				return printResult(nil, err, nil)
			}
			defer migrator.Driver.Close()

			rolledBack, err := migrator.Reset()
			if err != nil {
				return printResult(map[string]any{"rolled_back": rolledBack}, err, nil)
			}
			applied, err := migrator.Migrate()
			data := map[string]any{"rolled_back": rolledBack, "applied": applied}
			return printResult(data, err, func() {
				for _, name := range rolledBack {
					fmt.Printf("Rolling back migration: %s\n", name)
				}
				for _, name := range applied {
					fmt.Printf("Applying migration: %s\n", name)
				}
			})
		},
	}

	rootCmd.AddCommand(setupCmd, createCmd, migrateCmd, rollbackCmd, resetCmd, statusCmd, freshCmd)

	var refreshCmd = &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the database (Reset and Migrate)",
		RunE:  freshCmd.RunE,
	}
	rootCmd.AddCommand(refreshCmd)

	return rootCmd
}

func main() {
	if err := NewRootCmd().Execute(); err != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}
