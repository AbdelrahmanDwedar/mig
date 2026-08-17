package parser_test

import (
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/parser"
	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

func TestRegistry_For_UnsupportedExtension(t *testing.T) {
	dialect, err := sqlgen.New("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	reg := parser.NewRegistry(dialect)

	if _, err := reg.For("notes.txt"); err == nil {
		t.Fatal("expected error for unsupported migration file extension, got nil")
	}
}

func TestRegistry_IsMigrationFile(t *testing.T) {
	dialect, _ := sqlgen.New("sqlite")
	reg := parser.NewRegistry(dialect)

	if !reg.IsMigrationFile("2026_01_01_000000_x.sql") {
		t.Error("expected .sql to be recognized as a migration file")
	}
	if !reg.IsMigrationFile("2026_01_01_000000_x.json") {
		t.Error("expected .json to be recognized as a migration file")
	}
	if reg.IsMigrationFile("README.md") {
		t.Error("expected .md to not be recognized as a migration file")
	}
}
