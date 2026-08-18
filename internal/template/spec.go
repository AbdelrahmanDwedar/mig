package template

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

// typeSpecRe matches a column type token with optional length or
// precision/scale arguments: "string", "string(255)", "decimal(10,2)".
var typeSpecRe = regexp.MustCompile(`^(\w+)(?:\((\d+)(?:,(\d+))?\))?$`)

// ParseColumns parses a comma-separated column spec (as passed to
// --columns) into sqlgen.Column values. An empty spec returns an empty
// slice, not an error — callers that require at least one column check
// the length themselves.
func ParseColumns(spec string) ([]sqlgen.Column, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	fields := splitTopLevel(spec, ',')
	cols := make([]sqlgen.Column, 0, len(fields))
	for _, f := range fields {
		col, err := ParseColumn(f)
		if err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, nil
}

// ParseColumn parses a single column field of the form
// "name:type[(len)|(precision,scale)][:modifier]*". Recognized modifiers:
// pk, unique, auto, null, notnull, default=X, defaultexpr=X.
func ParseColumn(field string) (sqlgen.Column, error) {
	field = strings.TrimSpace(field)
	parts := strings.Split(field, ":")
	if len(parts) < 2 {
		return sqlgen.Column{}, fmt.Errorf(`invalid column spec %q: expected "name:type[:modifier...]"`, field)
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return sqlgen.Column{}, fmt.Errorf("invalid column spec %q: empty column name", field)
	}

	typeSpec := strings.TrimSpace(parts[1])
	m := typeSpecRe.FindStringSubmatch(typeSpec)
	if m == nil {
		return sqlgen.Column{}, fmt.Errorf("invalid column spec %q: malformed type %q", field, typeSpec)
	}
	typeName := m[1]
	if !sqlgen.IsValidAbstractType(typeName) {
		return sqlgen.Column{}, fmt.Errorf("invalid column spec %q: unknown type %q", field, typeName)
	}

	col := sqlgen.Column{Name: name, Type: typeName}
	if m[2] != "" {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			return sqlgen.Column{}, fmt.Errorf("invalid column spec %q: %w", field, err)
		}
		if m[3] != "" {
			scale, err := strconv.Atoi(m[3])
			if err != nil {
				return sqlgen.Column{}, fmt.Errorf("invalid column spec %q: %w", field, err)
			}
			col.Precision, col.Scale = n, scale
		} else {
			col.Length = n
		}
	}

	for _, mod := range parts[2:] {
		mod = strings.TrimSpace(mod)
		if mod == "" {
			continue
		}
		switch {
		case mod == "pk":
			col.PrimaryKey = true
		case mod == "unique":
			col.Unique = true
		case mod == "auto":
			col.AutoIncrement = true
		case mod == "null":
			v := true
			col.Nullable = &v
		case mod == "notnull":
			v := false
			col.Nullable = &v
		case strings.HasPrefix(mod, "default="):
			col.Default = parseDefaultLiteral(strings.TrimPrefix(mod, "default="))
		case strings.HasPrefix(mod, "defaultexpr="):
			col.Default = &sqlgen.Default{Expr: strings.TrimPrefix(mod, "defaultexpr=")}
		default:
			return sqlgen.Column{}, fmt.Errorf("invalid column spec %q: unknown modifier %q", field, mod)
		}
	}

	return col, nil
}

func parseDefaultLiteral(v string) *sqlgen.Default {
	switch strings.ToLower(v) {
	case "true":
		return &sqlgen.Default{Literal: true}
	case "false":
		return &sqlgen.Default{Literal: false}
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return &sqlgen.Default{Literal: f}
	}
	return &sqlgen.Default{Literal: v}
}

// splitTopLevel splits s on sep, ignoring occurrences of sep nested inside
// parentheses — so "decimal(10,2)" isn't split on its internal comma.
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// ParseIndex parses --index/--index-name/--index-unique into a sqlgen.Index.
// If name is empty, one is derived from the column list.
func ParseIndex(cols, name string, unique bool) (sqlgen.Index, error) {
	cols = strings.TrimSpace(cols)
	if cols == "" {
		return sqlgen.Index{}, fmt.Errorf("add_index requires --index \"col1,col2\"")
	}

	var columns []string
	for _, c := range strings.Split(cols, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			return sqlgen.Index{}, fmt.Errorf("invalid --index %q: empty column name", cols)
		}
		columns = append(columns, c)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = "idx_" + strings.Join(columns, "_")
	}

	return sqlgen.Index{Name: name, Columns: columns, Unique: unique}, nil
}

// fkReferencesRe matches "table(col1,col2)" as passed to --fk-references.
var fkReferencesRe = regexp.MustCompile(`^(\w+)\((\w+(?:\s*,\s*\w+)*)\)$`)

// ParseForeignKey parses --fk-column/--fk-references/--fk-on-delete/
// --fk-on-update/--fk-name into a sqlgen.ForeignKey. If name is empty, one
// is derived from the source columns and referenced table.
func ParseForeignKey(columns []string, references, onDelete, onUpdate, name string) (sqlgen.ForeignKey, error) {
	if len(columns) == 0 {
		return sqlgen.ForeignKey{}, fmt.Errorf("add_foreign_key requires at least one --fk-column")
	}

	references = strings.TrimSpace(references)
	m := fkReferencesRe.FindStringSubmatch(references)
	if m == nil {
		return sqlgen.ForeignKey{}, fmt.Errorf(`invalid --fk-references %q: expected "table(col1,col2)"`, references)
	}
	refTable := m[1]
	var refCols []string
	for _, c := range strings.Split(m[2], ",") {
		refCols = append(refCols, strings.TrimSpace(c))
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = "fk_" + strings.Join(columns, "_") + "_" + refTable
	}

	return sqlgen.ForeignKey{
		Name:       name,
		Columns:    columns,
		RefTable:   refTable,
		RefColumns: refCols,
		OnDelete:   onDelete,
		OnUpdate:   onUpdate,
	}, nil
}
