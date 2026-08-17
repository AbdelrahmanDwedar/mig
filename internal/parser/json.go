package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

// JSONParser implements Parser for the JSON migration language: a
// {"up": [...], "down": [...]} envelope of structured ops that compile down
// to dialect-native SQL via sqlgen.
type JSONParser struct {
	Dialect sqlgen.Dialect
}

func NewJSONParser(dialect sqlgen.Dialect) *JSONParser {
	return &JSONParser{Dialect: dialect}
}

func (p *JSONParser) Parse(content string) (up, down string, err error) {
	var migration sqlgen.Migration
	if err := json.Unmarshal([]byte(content), &migration); err != nil {
		return "", "", fmt.Errorf("invalid JSON migration: %w", err)
	}

	up, err = p.buildSection(migration.Up)
	if err != nil {
		return "", "", fmt.Errorf("up: %w", err)
	}
	down, err = p.buildSection(migration.Down)
	if err != nil {
		return "", "", fmt.Errorf("down: %w", err)
	}
	return up, down, nil
}

func (p *JSONParser) buildSection(ops []json.RawMessage) (string, error) {
	var statements []string
	for i, op := range ops {
		stmts, err := sqlgen.BuildStatement(p.Dialect, op)
		if err != nil {
			return "", fmt.Errorf("op %d: %w", i, err)
		}
		statements = append(statements, stmts...)
	}
	return strings.Join(statements, ";\n") + suffixIfNonEmpty(statements), nil
}

func suffixIfNonEmpty(statements []string) string {
	if len(statements) == 0 {
		return ""
	}
	return ";"
}
