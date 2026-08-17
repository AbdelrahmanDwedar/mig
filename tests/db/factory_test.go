package db_test

import (
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/config"
	"github.com/AbdelrahmanDwedar/mig/internal/db"
)

func TestNewDriver(t *testing.T) {
	tests := []struct {
		driver  string
		wantErr bool
	}{
		{"postgresql", false},
		{"mysql", false},
		{"sqlite", false},
		{"oracle", true},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			d, err := db.NewDriver(&config.DatabaseConfig{Driver: tt.driver})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for driver %q, got nil", tt.driver)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewDriver(%q): unexpected error: %v", tt.driver, err)
			}
			if d == nil {
				t.Fatalf("NewDriver(%q): expected non-nil driver", tt.driver)
			}
		})
	}
}

func TestMockDriver_ConnectClose(t *testing.T) {
	m := &db.MockDriver{}
	if err := m.Connect(); err != nil {
		t.Errorf("Connect: unexpected error: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
}

func TestMockDriver_RollbackMigration_NotFound(t *testing.T) {
	m := &db.MockDriver{}
	if err := m.RollbackMigration("does_not_exist.sql", ""); err == nil {
		t.Fatal("expected error rolling back a migration that was never applied, got nil")
	}
}
