package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AbdelrahmanDwedar/mig/internal/db"
	"github.com/AbdelrahmanDwedar/mig/internal/parser"
)

type Migrator struct {
	Driver   db.Driver
	Registry *parser.Registry
	Dir      string
}

func (m *Migrator) Migrate() ([]string, error) {
	if err := m.Driver.EnsureMigrationsTable(); err != nil {
		return nil, err
	}

	applied, err := m.Driver.GetAppliedMigrations()
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(m.Dir)
	if err != nil {
		return nil, err
	}

	var pending []string
	for _, f := range files {
		if !f.IsDir() && m.Registry.IsMigrationFile(f.Name()) {
			isApplied := false
			for _, a := range applied {
				if a == f.Name() {
					isApplied = true
					break
				}
			}
			if !isApplied {
				pending = append(pending, f.Name())
			}
		}
	}
	sort.Strings(pending)

	var appliedNow []string
	for _, name := range pending {
		content, err := os.ReadFile(filepath.Join(m.Dir, name))
		if err != nil {
			return appliedNow, err
		}
		p, err := m.Registry.For(name)
		if err != nil {
			return appliedNow, err
		}
		up, _, err := p.Parse(string(content))
		if err != nil {
			return appliedNow, err
		}
		if err := m.Driver.ApplyMigration(name, up); err != nil {
			return appliedNow, err
		}
		appliedNow = append(appliedNow, name)
	}
	return appliedNow, nil
}

func (m *Migrator) Rollback(steps int, migrationPath string) ([]string, error) {
	applied, err := m.Driver.GetAppliedMigrations()
	if err != nil {
		return nil, err
	}
	if len(applied) == 0 {
		return nil, fmt.Errorf("no migrations to rollback")
	}
	sort.Strings(applied)

	var targets []string
	if migrationPath != "" {
		found := false
		for _, a := range applied {
			if strings.Contains(a, migrationPath) {
				targets = append(targets, a)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("migration not found or not applied: %s", migrationPath)
		}
	} else {
		if steps <= 0 {
			steps = 1
		}
		if steps > len(applied) {
			steps = len(applied)
		}
		for i := 0; i < steps; i++ {
			targets = append(targets, applied[len(applied)-1-i])
		}
	}

	var rolledBack []string
	for _, name := range targets {
		content, err := os.ReadFile(filepath.Join(m.Dir, name))
		if err != nil {
			return rolledBack, err
		}

		p, err := m.Registry.For(name)
		if err != nil {
			return rolledBack, err
		}
		_, down, err := p.Parse(string(content))
		if err != nil {
			return rolledBack, err
		}

		if err := m.Driver.RollbackMigration(name, down); err != nil {
			return rolledBack, err
		}
		rolledBack = append(rolledBack, name)
	}
	return rolledBack, nil
}

func (m *Migrator) Reset() ([]string, error) {
	applied, err := m.Driver.GetAppliedMigrations()
	if err != nil {
		return nil, err
	}
	sort.Strings(applied)
	var rolledBack []string
	for i := len(applied) - 1; i >= 0; i-- {
		name := applied[i]
		content, err := os.ReadFile(filepath.Join(m.Dir, name))
		if err != nil {
			return rolledBack, err
		}
		p, err := m.Registry.For(name)
		if err != nil {
			return rolledBack, err
		}
		_, down, err := p.Parse(string(content))
		if err != nil {
			return rolledBack, err
		}
		if err := m.Driver.RollbackMigration(name, down); err != nil {
			return rolledBack, err
		}
		rolledBack = append(rolledBack, name)
	}
	return rolledBack, nil
}

func (m *Migrator) Status() ([]map[string]string, error) {
	applied, err := m.Driver.GetAppliedMigrations()
	if err != nil {
		return nil, err
	}
	appliedMap := make(map[string]bool)
	for _, a := range applied {
		appliedMap[a] = true
	}

	files, err := os.ReadDir(m.Dir)
	if err != nil {
		return nil, err
	}

	var allMigrations []string
	for _, f := range files {
		if !f.IsDir() && m.Registry.IsMigrationFile(f.Name()) {
			allMigrations = append(allMigrations, f.Name())
		}
	}
	sort.Strings(allMigrations)

	var status []map[string]string
	for _, name := range allMigrations {
		s := "Pending"
		if appliedMap[name] {
			s = "Applied"
		}
		status = append(status, map[string]string{"name": name, "status": s})
	}
	return status, nil
}
