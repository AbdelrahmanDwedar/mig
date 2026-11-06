package parser_test

import (
	"os"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/parser"
	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

func TestYAMLParser_Parse(t *testing.T) {
	content, err := os.ReadFile("../fixtures/migrations/full_example.yaml")
	if err != nil {
		t.Fatal(err)
	}

	dialect, err := sqlgen.New("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.NewYAMLParser(dialect)

	up, down, err := p.Parse(string(content))
	if err != nil {
		t.Fatal(err)
	}

	expectedUp := "CREATE TABLE \"users\" (\n  \"id\" INTEGER PRIMARY KEY AUTOINCREMENT,\n  \"email\" VARCHAR(255) NOT NULL UNIQUE\n);\n" +
		"ALTER TABLE \"users\" ADD COLUMN \"plan\" VARCHAR(50) DEFAULT 'free';"
	expectedDown := "ALTER TABLE \"users\" DROP COLUMN \"plan\";\n" +
		"DROP TABLE IF EXISTS \"users\";"

	if up != expectedUp {
		t.Errorf("up mismatch:\ngot:  %s\nwant: %s", up, expectedUp)
	}
	if down != expectedDown {
		t.Errorf("down mismatch:\ngot:  %s\nwant: %s", down, expectedDown)
	}
}

func TestYAMLParser_Parse_InvalidYAML(t *testing.T) {
	dialect, _ := sqlgen.New("sqlite")
	p := parser.NewYAMLParser(dialect)
	if _, _, err := p.Parse("up:\n  - op: create_table\n\ttable: users\n"); err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestYAMLParser_Parse_MultiDocument(t *testing.T) {
	dialect, _ := sqlgen.New("sqlite")
	p := parser.NewYAMLParser(dialect)
	if _, _, err := p.Parse("up: []\ndown: []\n---\nup: []\ndown: []\n"); err == nil {
		t.Fatal("expected error for multi-document YAML, got nil")
	}
}

func TestYAMLParser_Parse_EmptySections(t *testing.T) {
	dialect, _ := sqlgen.New("sqlite")
	p := parser.NewYAMLParser(dialect)
	up, down, err := p.Parse("up: []\ndown: []\n")
	if err != nil {
		t.Fatal(err)
	}
	if up != "" || down != "" {
		t.Errorf("expected empty up/down for empty op lists, got up=%q down=%q", up, down)
	}
}

func TestYAMLParser_Parse_DownOpError(t *testing.T) {
	dialect, _ := sqlgen.New("sqlite")
	p := parser.NewYAMLParser(dialect)
	if _, _, err := p.Parse("up: []\ndown:\n  - op: frobnicate\n"); err == nil {
		t.Fatal("expected error for unknown op in down section, got nil")
	}
}

func TestYAMLParser_Parse_UnknownOp(t *testing.T) {
	dialect, _ := sqlgen.New("sqlite")
	p := parser.NewYAMLParser(dialect)
	if _, _, err := p.Parse("up:\n  - op: frobnicate\ndown: []\n"); err == nil {
		t.Fatal("expected error for unknown op, got nil")
	}
}
