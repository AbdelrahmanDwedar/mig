package parser_test

import (
	"os"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/parser"
	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

func TestJSONParser_Parse(t *testing.T) {
	content, err := os.ReadFile("../fixtures/migrations/full_example.json")
	if err != nil {
		t.Fatal(err)
	}

	dialect, err := sqlgen.New("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.NewJSONParser(dialect)

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

func TestJSONParser_Parse_InvalidJSON(t *testing.T) {
	dialect, _ := sqlgen.New("sqlite")
	p := parser.NewJSONParser(dialect)
	if _, _, err := p.Parse("{not json"); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestJSONParser_Parse_UnknownOp(t *testing.T) {
	dialect, _ := sqlgen.New("sqlite")
	p := parser.NewJSONParser(dialect)
	if _, _, err := p.Parse(`{"up": [{"op": "frobnicate"}], "down": []}`); err == nil {
		t.Fatal("expected error for unknown op, got nil")
	}
}
