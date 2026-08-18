package parser

import (
	"fmt"
	"path/filepath"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

// Registry dispatches a migration file to the Parser matching its
// extension, so .sql, .json, and .yaml/.yml migrations can coexist in the
// same migrations directory.
type Registry struct {
	parsers map[string]Parser
}

func NewRegistry(dialect sqlgen.Dialect) *Registry {
	yamlParser := NewYAMLParser(dialect)
	return &Registry{
		parsers: map[string]Parser{
			".sql":  &SQLParser{},
			".json": NewJSONParser(dialect),
			".yaml": yamlParser,
			".yml":  yamlParser,
		},
	}
}

// IsMigrationFile reports whether filename has an extension this registry
// knows how to parse.
func (r *Registry) IsMigrationFile(filename string) bool {
	_, ok := r.parsers[filepath.Ext(filename)]
	return ok
}

// For returns the Parser registered for filename's extension.
func (r *Registry) For(filename string) (Parser, error) {
	ext := filepath.Ext(filename)
	p, ok := r.parsers[ext]
	if !ok {
		return nil, fmt.Errorf("no parser registered for migration file extension %q (%s)", ext, filename)
	}
	return p, nil
}
