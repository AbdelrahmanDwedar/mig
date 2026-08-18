package parser

import (
	"encoding/json"
	"fmt"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
	"github.com/AbdelrahmanDwedar/mig/internal/yamlconv"
)

// YAMLParser implements Parser for the YAML migration language: the same
// {"up": [...], "down": [...]} envelope of structured ops as the JSON
// language, written as YAML. It converts to JSON and reuses the exact same
// sqlgen op machinery as JSONParser.
type YAMLParser struct {
	Dialect sqlgen.Dialect
}

func NewYAMLParser(dialect sqlgen.Dialect) *YAMLParser {
	return &YAMLParser{Dialect: dialect}
}

func (p *YAMLParser) Parse(content string) (up, down string, err error) {
	jsonBytes, err := yamlconv.YAMLToJSON([]byte(content))
	if err != nil {
		return "", "", fmt.Errorf("invalid YAML migration: %w", err)
	}

	var migration sqlgen.Migration
	if err := json.Unmarshal(jsonBytes, &migration); err != nil {
		return "", "", fmt.Errorf("invalid YAML migration: %w", err)
	}

	up, err = buildSection(p.Dialect, migration.Up)
	if err != nil {
		return "", "", fmt.Errorf("up: %w", err)
	}
	down, err = buildSection(p.Dialect, migration.Down)
	if err != nil {
		return "", "", fmt.Errorf("down: %w", err)
	}
	return up, down, nil
}
