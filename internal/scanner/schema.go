package scanner

import "github.com/AbdelrahmanDwedar/mig/internal/sqlgen"

// RawType is the sentinel sqlgen.Column.Type value used when a scanned
// column's native type has no portable abstract equivalent (e.g. a
// Postgres-specific type, a MySQL ENUM(...), or an unrecognized affinity).
// The actual native type string is preserved on Column.NativeType so
// nothing the database reports is silently lost.
const RawType = "raw"

// Schema is the aggregate, in-memory model of everything a Scanner found in
// a database.
type Schema struct {
	Tables    map[string]*Table    `json:"tables"`
	Views     map[string]*View     `json:"views,omitempty"`
	Triggers  map[string]*Trigger  `json:"triggers,omitempty"`
	Sequences map[string]*Sequence `json:"sequences,omitempty"`
}

// Table describes one scanned table. Columns/ForeignKeys/Checks reuse the
// same portable types the JSON migration format uses (internal/sqlgen), so
// a scanned Schema can be fed directly into future diff/generate work
// without a translation layer.
type Table struct {
	Name        string              `json:"name"`
	Columns     []Column            `json:"columns"`
	Indexes     []Index             `json:"indexes,omitempty"`
	ForeignKeys []sqlgen.ForeignKey `json:"foreign_keys,omitempty"`
	Checks      []sqlgen.Check      `json:"checks,omitempty"`
}

// Column wraps sqlgen.Column with the raw-native-type fallback described by
// RawType.
type Column struct {
	sqlgen.Column
	NativeType string `json:"native_type,omitempty"`
}

// Index wraps sqlgen.Index with dialect-specific extras a portable index
// definition can't express: the index method/type (e.g. Postgres
// btree/gin/gist/brin), a partial-index predicate, and a catch-all bag for
// anything else worth surfacing (e.g. MySQL FULLTEXT/SPATIAL).
type Index struct {
	sqlgen.Index
	Method  string            `json:"method,omitempty"`
	Partial string            `json:"partial,omitempty"`
	Extra   map[string]string `json:"extra,omitempty"`
}

// View describes a scanned database view.
type View struct {
	Name       string `json:"name"`
	Definition string `json:"definition,omitempty"`
}

// Trigger describes a scanned database trigger.
type Trigger struct {
	Name       string `json:"name"`
	Table      string `json:"table"`
	Timing     string `json:"timing"`
	Event      string `json:"event"`
	Definition string `json:"definition,omitempty"`
}

// Sequence describes a scanned standalone sequence object. Not every
// dialect has these (SQLite doesn't) — a Scanner returns an empty map
// rather than an error for dialects without sequence support.
type Sequence struct {
	Name      string `json:"name"`
	Start     int64  `json:"start"`
	Increment int64  `json:"increment"`
	MinValue  int64  `json:"min_value,omitempty"`
	MaxValue  int64  `json:"max_value,omitempty"`
}
