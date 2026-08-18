// Package template scaffolds common migration operations (create table,
// add column, add index, ...) for `mig create --template`. It builds on
// the same op vocabulary and Dialect abstraction the JSON migration
// language uses (internal/sqlgen), so scaffolded SQL is rendered via the
// exact Dialect methods invoked at migrate-time, and scaffolded JSON is
// the same {"op": ...} envelope the JSON parser consumes.
package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

// Op is the result of resolving a --template invocation: a discriminator
// matching sqlgen's JSON "op" values, plus the up-direction struct.
type Op struct {
	Kind string
	Up   any
}

// Flags carries the raw CLI flag values BuildOp consults. Only the subset
// relevant to a given template's Kind need be set.
type Flags struct {
	Table        string
	Columns      string
	Column       string
	To           string
	IndexColumns string
	IndexName    string
	IndexUnique  bool
	FKColumns    []string
	FKReferences string
	FKOnDelete   string
	FKOnUpdate   string
	FKName       string
	IfNotExists  bool
	IfExists     bool
}

// SupportedTemplates lists the template kinds `mig create --template` accepts.
var SupportedTemplates = []string{
	"create_table", "drop_table",
	"add_column", "drop_column",
	"add_index", "drop_index",
	"rename_table", "rename_column",
	"add_foreign_key", "drop_foreign_key",
}

// IsSupported reports whether kind is a recognized template.
func IsSupported(kind string) bool {
	for _, k := range SupportedTemplates {
		if k == kind {
			return true
		}
	}
	return false
}

// BuildOp validates kind and the flags required for it, then constructs the
// matching sqlgen op struct describing the "up" direction.
func BuildOp(kind string, f Flags) (Op, error) {
	if !IsSupported(kind) {
		return Op{}, fmt.Errorf("unknown template %q (supported: %s)", kind, strings.Join(SupportedTemplates, ", "))
	}
	if f.Table == "" {
		return Op{}, fmt.Errorf("--template %s requires --table", kind)
	}

	switch kind {
	case "create_table":
		cols, err := ParseColumns(f.Columns)
		if err != nil {
			return Op{}, err
		}
		if len(cols) == 0 {
			return Op{}, fmt.Errorf("--template create_table requires --columns")
		}
		return Op{Kind: kind, Up: sqlgen.CreateTableOp{Table: f.Table, IfNotExists: f.IfNotExists, Columns: cols}}, nil

	case "drop_table":
		return Op{Kind: kind, Up: sqlgen.DropTableOp{Table: f.Table, IfExists: f.IfExists}}, nil

	case "add_column":
		if f.Columns == "" {
			return Op{}, fmt.Errorf(`--template add_column requires --columns "name:type[:modifier...]"`)
		}
		cols, err := ParseColumns(f.Columns)
		if err != nil {
			return Op{}, err
		}
		if len(cols) != 1 {
			return Op{}, fmt.Errorf("--template add_column takes exactly one column, got %d", len(cols))
		}
		return Op{Kind: kind, Up: sqlgen.AddColumnOp{Table: f.Table, Column: cols[0]}}, nil

	case "drop_column":
		if f.Column == "" {
			return Op{}, fmt.Errorf("--template drop_column requires --column <name>")
		}
		return Op{Kind: kind, Up: sqlgen.DropColumnOp{Table: f.Table, Column: f.Column}}, nil

	case "rename_column":
		if f.Column == "" || f.To == "" {
			return Op{}, fmt.Errorf("--template rename_column requires --column <old name> and --to <new name>")
		}
		return Op{Kind: kind, Up: sqlgen.RenameColumnOp{Table: f.Table, From: f.Column, To: f.To}}, nil

	case "rename_table":
		if f.To == "" {
			return Op{}, fmt.Errorf("--template rename_table requires --to <new name>")
		}
		return Op{Kind: kind, Up: sqlgen.RenameTableOp{From: f.Table, To: f.To}}, nil

	case "add_index":
		idx, err := ParseIndex(f.IndexColumns, f.IndexName, f.IndexUnique)
		if err != nil {
			return Op{}, err
		}
		return Op{Kind: kind, Up: sqlgen.AddIndexOp{Table: f.Table, Index: idx}}, nil

	case "drop_index":
		if f.IndexName == "" {
			return Op{}, fmt.Errorf("--template drop_index requires --index-name <name>")
		}
		return Op{Kind: kind, Up: sqlgen.DropIndexOp{Table: f.Table, Name: f.IndexName}}, nil

	case "add_foreign_key":
		fk, err := ParseForeignKey(f.FKColumns, f.FKReferences, f.FKOnDelete, f.FKOnUpdate, f.FKName)
		if err != nil {
			return Op{}, err
		}
		return Op{Kind: kind, Up: sqlgen.AddForeignKeyOp{Table: f.Table, ForeignKey: fk}}, nil

	case "drop_foreign_key":
		if f.FKName == "" {
			return Op{}, fmt.Errorf("--template drop_foreign_key requires --fk-name <name>")
		}
		return Op{Kind: kind, Up: sqlgen.DropForeignKeyOp{Table: f.Table, Name: f.FKName}}, nil
	}

	return Op{}, fmt.Errorf("unhandled template %q", kind)
}

// todoStub builds the down-side placeholder for ops whose original shape
// isn't recoverable from the up invocation alone. It's expressed as the
// "sql" escape hatch so it stays valid in both SQL (a comment line) and
// JSON (a schema-valid op) output.
func todoStub(kind, table string) Op {
	return Op{Kind: "sql", Up: sqlgen.SQLOp{
		Query: fmt.Sprintf("-- TODO: cannot auto-reverse %s on %q; write the down migration manually", kind, table),
	}}
}

// Down derives the down-direction Op from an up Op. Additive and rename
// ops are mechanically invertible from their own fields; destructive ops
// (drop_table/drop_column/drop_index/drop_foreign_key) can't recover the
// shape they destroyed, so they get a TODO stub instead. This one rule
// applies uniformly — no per-op special casing beyond it.
func Down(op Op) Op {
	switch up := op.Up.(type) {
	case sqlgen.CreateTableOp:
		return Op{Kind: "drop_table", Up: sqlgen.DropTableOp{Table: up.Table}}
	case sqlgen.DropTableOp:
		return todoStub(op.Kind, up.Table)
	case sqlgen.AddColumnOp:
		return Op{Kind: "drop_column", Up: sqlgen.DropColumnOp{Table: up.Table, Column: up.Column.Name}}
	case sqlgen.DropColumnOp:
		return todoStub(op.Kind, up.Table)
	case sqlgen.RenameTableOp:
		return Op{Kind: "rename_table", Up: sqlgen.RenameTableOp{From: up.To, To: up.From}}
	case sqlgen.RenameColumnOp:
		return Op{Kind: "rename_column", Up: sqlgen.RenameColumnOp{Table: up.Table, From: up.To, To: up.From}}
	case sqlgen.AddIndexOp:
		return Op{Kind: "drop_index", Up: sqlgen.DropIndexOp{Table: up.Table, Name: up.Index.Name}}
	case sqlgen.DropIndexOp:
		return todoStub(op.Kind, up.Table)
	case sqlgen.AddForeignKeyOp:
		return Op{Kind: "drop_foreign_key", Up: sqlgen.DropForeignKeyOp{Table: up.Table, Name: up.ForeignKey.Name}}
	case sqlgen.DropForeignKeyOp:
		return todoStub(op.Kind, up.Table)
	default:
		return todoStub(op.Kind, "")
	}
}

// dialectDispatch maps a template Kind to the Dialect method that renders
// it — the same methods sqlgen.BuildStatement calls for JSON migrations.
var dialectDispatch = map[string]func(sqlgen.Dialect, any) ([]string, error){
	"create_table":     func(d sqlgen.Dialect, v any) ([]string, error) { return d.CreateTable(v.(sqlgen.CreateTableOp)) },
	"drop_table":       func(d sqlgen.Dialect, v any) ([]string, error) { return d.DropTable(v.(sqlgen.DropTableOp)) },
	"rename_table":     func(d sqlgen.Dialect, v any) ([]string, error) { return d.RenameTable(v.(sqlgen.RenameTableOp)) },
	"add_column":       func(d sqlgen.Dialect, v any) ([]string, error) { return d.AddColumn(v.(sqlgen.AddColumnOp)) },
	"drop_column":      func(d sqlgen.Dialect, v any) ([]string, error) { return d.DropColumn(v.(sqlgen.DropColumnOp)) },
	"rename_column":    func(d sqlgen.Dialect, v any) ([]string, error) { return d.RenameColumn(v.(sqlgen.RenameColumnOp)) },
	"add_index":        func(d sqlgen.Dialect, v any) ([]string, error) { return d.AddIndex(v.(sqlgen.AddIndexOp)) },
	"drop_index":       func(d sqlgen.Dialect, v any) ([]string, error) { return d.DropIndex(v.(sqlgen.DropIndexOp)) },
	"add_foreign_key":  func(d sqlgen.Dialect, v any) ([]string, error) { return d.AddForeignKey(v.(sqlgen.AddForeignKeyOp)) },
	"drop_foreign_key": func(d sqlgen.Dialect, v any) ([]string, error) { return d.DropForeignKey(v.(sqlgen.DropForeignKeyOp)) },
}

func renderOne(dialect sqlgen.Dialect, op Op) ([]string, error) {
	if op.Kind == "sql" {
		return []string{op.Up.(sqlgen.SQLOp).Query}, nil
	}
	fn, ok := dialectDispatch[op.Kind]
	if !ok {
		return nil, fmt.Errorf("unknown template op: %q", op.Kind)
	}
	return fn(dialect, op.Up)
}

func joinStatements(stmts []string) string {
	if len(stmts) == 0 {
		return ""
	}
	return strings.Join(stmts, ";\n") + ";"
}

// RenderSQL renders op and its derived down side as dialect-native SQL,
// via the same Dialect methods the JSON migration parser calls at
// migrate-time — so scaffolded SQL is guaranteed valid for the dialect (or
// fails with the dialect's real "unsupported" error, e.g. SQLite +
// add_foreign_key) rather than being independently hand-written text.
func RenderSQL(dialect sqlgen.Dialect, op Op) (string, error) {
	upStmts, err := renderOne(dialect, op)
	if err != nil {
		return "", fmt.Errorf("up: %w", err)
	}
	downStmts, err := renderOne(dialect, Down(op))
	if err != nil {
		return "", fmt.Errorf("down: %w", err)
	}

	var b strings.Builder
	b.WriteString("-- +migrate Up\n")
	b.WriteString(joinStatements(upStmts))
	b.WriteString("\n\n-- +migrate Down\n")
	b.WriteString(joinStatements(downStmts))
	b.WriteString("\n")
	return b.String(), nil
}

// RenderJSON serializes op and its derived down side into the
// {"up":[...],"down":[...]} envelope JSONParser consumes. No dialect is
// needed here — dialect resolution for JSON migrations happens later, at
// migrate-time.
func RenderJSON(op Op) (string, error) {
	upRaw, err := marshalOp(op)
	if err != nil {
		return "", fmt.Errorf("up: %w", err)
	}
	downRaw, err := marshalOp(Down(op))
	if err != nil {
		return "", fmt.Errorf("down: %w", err)
	}

	envelope := struct {
		Up   []json.RawMessage `json:"up"`
		Down []json.RawMessage `json:"down"`
	}{Up: []json.RawMessage{upRaw}, Down: []json.RawMessage{downRaw}}

	b, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

func marshalOp(op Op) (json.RawMessage, error) {
	b, err := json.Marshal(op.Up)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	kindJSON, err := json.Marshal(op.Kind)
	if err != nil {
		return nil, err
	}
	m["op"] = kindJSON
	return json.Marshal(m)
}
