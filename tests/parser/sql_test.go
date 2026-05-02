package parser_test

import (
	"testing"

	"github.com/mig-tool/mig/internal/parser"
)

func TestSQLParser_Parse(t *testing.T) {
	parser := &parser.SQLParser{}
	content := `-- +migrate Up
CREATE TABLE test (id INT);

-- +migrate Down
DROP TABLE test;
`
	up, down, err := parser.Parse(content)
	if err != nil {
		t.Fatal(err)
	}

	expectedUp := "CREATE TABLE test (id INT);"
	expectedDown := "DROP TABLE test;"

	if up != expectedUp {
		t.Errorf("Expected UP: %s, got: %s", expectedUp, up)
	}
	if down != expectedDown {
		t.Errorf("Expected DOWN: %s, got: %s", expectedDown, down)
	}
}
