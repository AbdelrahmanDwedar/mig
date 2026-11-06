package scanner

import "github.com/AbdelrahmanDwedar/mig/internal/sqlgen"

const RawType = "raw"

type Schema struct {
	Tables    map[string]*Table    `json:"tables"`
	Views     map[string]*View     `json:"views,omitempty"`
	Triggers  map[string]*Trigger  `json:"triggers,omitempty"`
	Sequences map[string]*Sequence `json:"sequences,omitempty"`
}

type Table struct {
	Name        string              `json:"name"`
	Columns     []Column            `json:"columns"`
	Indexes     []Index             `json:"indexes,omitempty"`
	ForeignKeys []sqlgen.ForeignKey `json:"foreign_keys,omitempty"`
	Checks      []sqlgen.Check      `json:"checks,omitempty"`
}

type Column struct {
	sqlgen.Column
	NativeType string `json:"native_type,omitempty"`
}

type Index struct {
	sqlgen.Index
	Method  string            `json:"method,omitempty"`
	Partial string            `json:"partial,omitempty"`
	Extra   map[string]string `json:"extra,omitempty"`
}

type View struct {
	Name       string `json:"name"`
	Definition string `json:"definition,omitempty"`
}

type Trigger struct {
	Name       string `json:"name"`
	Table      string `json:"table"`
	Timing     string `json:"timing"`
	Event      string `json:"event"`
	Definition string `json:"definition,omitempty"`
}

type Sequence struct {
	Name      string `json:"name"`
	Start     int64  `json:"start"`
	Increment int64  `json:"increment"`
	MinValue  int64  `json:"min_value,omitempty"`
	MaxValue  int64  `json:"max_value,omitempty"`
}
