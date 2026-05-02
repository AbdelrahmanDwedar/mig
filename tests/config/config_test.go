package config_test

import (
	"os"
	"testing"

	"github.com/mig-tool/mig/internal/config"
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
