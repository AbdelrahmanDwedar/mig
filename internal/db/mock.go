package db

import (
	"database/sql"
	"errors"
)

type MockDriver struct {
	AppliedMigrations []string
	DB                *sql.DB
}

func (m *MockDriver) Connect() error               { return nil }
func (m *MockDriver) Close() error                 { return nil }
func (m *MockDriver) Conn() *sql.DB                { return m.DB }
func (m *MockDriver) EnsureMigrationsTable() error { return nil }
func (m *MockDriver) GetAppliedMigrations() ([]string, error) {
	return m.AppliedMigrations, nil
}
func (m *MockDriver) ApplyMigration(name, upSQL string) error {
	m.AppliedMigrations = append(m.AppliedMigrations, name)
	return nil
}
func (m *MockDriver) RollbackMigration(name, downSQL string) error {
	for i, v := range m.AppliedMigrations {
		if v == name {
			m.AppliedMigrations = append(m.AppliedMigrations[:i], m.AppliedMigrations[i+1:]...)
			return nil
		}
	}
	return errors.New("migration not found")
}
