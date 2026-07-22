package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/AbdelrahmanDwedar/mig/internal/config"
	"github.com/AbdelrahmanDwedar/mig/internal/db"
	"github.com/AbdelrahmanDwedar/mig/internal/migrate"
	"github.com/AbdelrahmanDwedar/mig/internal/parser"
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
  parser: sql
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
  parser: sql
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

func createMigration(name string) (string, error) {
	cfg, err := config.LoadConfig("mig.yml")
	dir := "migrations"
	if err == nil && cfg.Migrations.Dir != "" {
		dir = cfg.Migrations.Dir
	}

	timestamp := time.Now().Format("2006_01_02_150405")
	filename := fmt.Sprintf("%s/%s_%s.sql", dir, timestamp, name)

	if err := os.WriteFile(filename, []byte(migrationBoilerplate), 0644); err != nil {
		return "", fmt.Errorf("failed to create migration file: %w", err)
	}
	if !jsonOutput {
		fmt.Printf("Created migration: %s\n", filename)
	}
	return filename, nil
}

func getParser(parserType string) (parser.Parser, error) {
	switch parserType {
	case "sql", "": // default to sql
		return &parser.SQLParser{}, nil
	default:
		return nil, fmt.Errorf("unsupported parser: %s", parserType)
	}
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
	p, err := getParser(cfg.Migrations.Parser)
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

	return &migrate.Migrator{Driver: driver, Parser: p, Dir: dir}, nil
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

	var createCmd = &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new migration file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filename, err := createMigration(args[0])
			return printResult(map[string]any{"file": filename}, err, nil)
		},
	}

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
