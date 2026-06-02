package config_test

import (
	"os"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/config"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	content := []byte("database:\n  driver: sqlite\n  dbname: test.db\nmigrations:\n  parser: sql\n  dir: migrations\n")
	err := os.WriteFile("test_mig.yml", content, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test_mig.yml")

	cfg, err := config.LoadConfig("test_mig.yml")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Database.Driver != "sqlite" {
		t.Errorf("Expected driver sqlite, got %s", cfg.Database.Driver)
	}
	if cfg.Migrations.Dir != "migrations" {
		t.Errorf("Expected dir migrations, got %s", cfg.Migrations.Dir)
	}
}

func TestLoadConfigInterpolation(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		content     string
		expectedDB  string
		expectedDir string
		expectErr   bool
	}{
		{
			name: "Basic interpolation",
			env: map[string]string{
				"INTERPOLATE_DB_NAME": "prod_db",
				"INTERPOLATE_DIR":     "prod_migrations",
			},
			content:     "database:\n  dbname: ${INTERPOLATE_DB_NAME}\nmigrations:\n  dir: ${INTERPOLATE_DIR}\n",
			expectedDB:  "prod_db",
			expectedDir: "prod_migrations",
		},
		{
			name:        "Default values",
			env:         map[string]string{},
			content:     "database:\n  dbname: ${UNSET_DB_NAME:-default_db}\nmigrations:\n  dir: ${UNSET_DIR:-default_migrations}\n",
			expectedDB:  "default_db",
			expectedDir: "default_migrations",
		},
		{
			name:      "Mandatory variable - missing",
			env:       map[string]string{},
			content:   "database:\n  dbname: ${REQUIRED_DB_NAME:?database name is required}\n",
			expectErr: true,
		},
		{
			name: "Mandatory variable - present",
			env: map[string]string{
				"REQUIRED_DB_NAME": "required_db",
			},
			content:    "database:\n  dbname: ${REQUIRED_DB_NAME:?database name is required}\n",
			expectedDB: "required_db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unset variables first to ensure clean state
			for k := range tt.env {
				os.Unsetenv(k)
			}
			// Also unset variables that might be missing but used in content
			os.Unsetenv("UNSET_DB_NAME")
			os.Unsetenv("UNSET_DIR")
			os.Unsetenv("REQUIRED_DB_NAME")

			// Set environment variables
			for k, v := range tt.env {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			filename := "test_interpolation.yml"
			err := os.WriteFile(filename, []byte(tt.content), 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(filename)

			cfg, err := config.LoadConfig(filename)
			if (err != nil) != tt.expectErr {
				t.Errorf("LoadConfig() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tt.expectErr {
				return
			}

			if tt.expectedDB != "" && cfg.Database.DBName != tt.expectedDB {
				t.Errorf("Expected dbname %s, got %s", tt.expectedDB, cfg.Database.DBName)
			}
			if tt.expectedDir != "" && cfg.Migrations.Dir != tt.expectedDir {
				t.Errorf("Expected dir %s, got %s", tt.expectedDir, cfg.Migrations.Dir)
			}
		})
	}
}
