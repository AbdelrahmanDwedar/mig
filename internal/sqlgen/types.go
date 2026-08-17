package sqlgen

import "encoding/json"

// Default represents a column default value: either a quoted/escaped literal
// or a verbatim SQL expression (e.g. now(), gen_random_uuid()).
type Default struct {
	Literal interface{} `json:"literal,omitempty"`
	Expr    string      `json:"expr,omitempty"`
}

// Column describes a single column in a create_table or add_column op.
type Column struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Length        int      `json:"length,omitempty"`
	Precision     int      `json:"precision,omitempty"`
	Scale         int      `json:"scale,omitempty"`
	Nullable      *bool    `json:"nullable,omitempty"`
	PrimaryKey    bool     `json:"primary_key,omitempty"`
	Unique        bool     `json:"unique,omitempty"`
	AutoIncrement bool     `json:"auto_increment,omitempty"`
	Default       *Default `json:"default,omitempty"`
}

// Index describes a named index over one or more columns.
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique,omitempty"`
}

// ForeignKey describes a foreign key constraint.
type ForeignKey struct {
	Name       string   `json:"name,omitempty"`
	Columns    []string `json:"columns"`
	RefTable   string   `json:"ref_table"`
	RefColumns []string `json:"ref_columns"`
	OnDelete   string   `json:"on_delete,omitempty"`
	OnUpdate   string   `json:"on_update,omitempty"`
}

// Check describes a CHECK constraint.
type Check struct {
	Name string `json:"name,omitempty"`
	Expr string `json:"expr"`
}

type opEnvelope struct {
	Op string `json:"op"`
}

type CreateTableOp struct {
	Table       string       `json:"table"`
	IfNotExists bool         `json:"if_not_exists,omitempty"`
	Columns     []Column     `json:"columns"`
	Indexes     []Index      `json:"indexes,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
	Checks      []Check      `json:"checks,omitempty"`
}

type DropTableOp struct {
	Table    string `json:"table"`
	IfExists bool   `json:"if_exists,omitempty"`
}

type RenameTableOp struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type AddColumnOp struct {
	Table  string `json:"table"`
	Column Column `json:"column"`
}

type DropColumnOp struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

type RenameColumnOp struct {
	Table string `json:"table"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// AlterColumnOp uses pointer fields so "omitted" is distinguishable from
// "explicit zero value" — only the fields that are set get altered.
type AlterColumnOp struct {
	Table     string   `json:"table"`
	Column    string   `json:"column"`
	Type      *string  `json:"type,omitempty"`
	Length    *int     `json:"length,omitempty"`
	Precision *int     `json:"precision,omitempty"`
	Scale     *int     `json:"scale,omitempty"`
	Nullable  *bool    `json:"nullable,omitempty"`
	Default   *Default `json:"default,omitempty"`
}

type AddIndexOp struct {
	Table string `json:"table"`
	Index Index  `json:"index"`
}

type DropIndexOp struct {
	Table string `json:"table"`
	Name  string `json:"name"`
}

type AddForeignKeyOp struct {
	Table      string     `json:"table"`
	ForeignKey ForeignKey `json:"foreign_key"`
}

type DropForeignKeyOp struct {
	Table string `json:"table"`
	Name  string `json:"name"`
}

type AddConstraintOp struct {
	Table string `json:"table"`
	Check Check  `json:"check"`
}

type DropConstraintOp struct {
	Table string `json:"table"`
	Name  string `json:"name"`
}

type SQLOp struct {
	Query string `json:"query"`
}

// Migration is the top-level JSON envelope: {"up": [...], "down": [...]}.
// Each element is a raw op object dispatched by its "op" discriminator.
type Migration struct {
	Up   []json.RawMessage `json:"up"`
	Down []json.RawMessage `json:"down"`
}
