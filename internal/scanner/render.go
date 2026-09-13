package scanner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

// RenderText renders a Schema as a human-readable tree (├── / └──
// connectors, matching the style of `mig`'s own cobra --help command tree),
// the default (non --json) output of `mig inspect`.
func RenderText(schema *Schema) string {
	var b strings.Builder

	tableNames := sortedKeys(schema.Tables)
	fmt.Fprintf(&b, "Tables (%d)\n", len(tableNames))
	for i, name := range tableNames {
		renderTable(&b, "", isLastIndex(i, len(tableNames)), schema.Tables[name])
	}

	if len(schema.Views) > 0 {
		names := sortedKeys(schema.Views)
		fmt.Fprintf(&b, "\nViews (%d)\n", len(names))
		for i, name := range names {
			v := schema.Views[name]
			label := v.Name
			if v.Definition != "" {
				label = fmt.Sprintf("%s — %s", v.Name, oneLine(v.Definition))
			}
			writeLine(&b, "", isLastIndex(i, len(names)), label)
		}
	}

	if len(schema.Triggers) > 0 {
		names := sortedKeys(schema.Triggers)
		fmt.Fprintf(&b, "\nTriggers (%d)\n", len(names))
		for i, name := range names {
			t := schema.Triggers[name]
			writeLine(&b, "", isLastIndex(i, len(names)), fmt.Sprintf("%s (%s, %s %s)", t.Name, t.Table, t.Timing, t.Event))
		}
	}

	if len(schema.Sequences) > 0 {
		names := sortedKeys(schema.Sequences)
		fmt.Fprintf(&b, "\nSequences (%d)\n", len(names))
		for i, name := range names {
			s := schema.Sequences[name]
			writeLine(&b, "", isLastIndex(i, len(names)), fmt.Sprintf("%s (start %d, increment %d)", s.Name, s.Start, s.Increment))
		}
	}

	return b.String()
}

// renderTable writes a table as a tree node with four branches (Columns,
// Indexes, Foreign Keys, Checks), each fanning out into its own leaves.
func renderTable(b *strings.Builder, prefix string, isLast bool, t *Table) {
	writeLine(b, prefix, isLast, t.Name)
	tablePrefix := childPrefix(prefix, isLast)

	sections := []struct {
		label string
		lines []string
	}{
		{"Columns", columnLines(t.Columns)},
		{"Indexes", indexLines(t.Indexes)},
		{"Foreign Keys", fkLines(t.ForeignKeys)},
		{"Checks", checkLines(t.Checks)},
	}

	for i, sec := range sections {
		secIsLast := isLastIndex(i, len(sections))
		writeLine(b, tablePrefix, secIsLast, sec.label)
		secPrefix := childPrefix(tablePrefix, secIsLast)

		lines := sec.lines
		if len(lines) == 0 {
			lines = []string{"(none)"}
		}
		for j, line := range lines {
			writeLine(b, secPrefix, isLastIndex(j, len(lines)), line)
		}
	}
}

func columnLines(cols []Column) []string {
	lines := make([]string, len(cols))
	for i, c := range cols {
		lines[i] = renderColumn(c)
	}
	return lines
}

func indexLines(idxs []Index) []string {
	lines := make([]string, len(idxs))
	for i, idx := range idxs {
		lines[i] = renderIndex(idx)
	}
	return lines
}

func fkLines(fks []sqlgen.ForeignKey) []string {
	lines := make([]string, len(fks))
	for i, fk := range fks {
		line := fmt.Sprintf("%s (%s) -> %s (%s)", fkLabel(fk.Name), strings.Join(fk.Columns, ", "), fk.RefTable, strings.Join(fk.RefColumns, ", "))
		if fk.OnDelete != "" {
			line += " ON DELETE " + fk.OnDelete
		}
		if fk.OnUpdate != "" {
			line += " ON UPDATE " + fk.OnUpdate
		}
		lines[i] = line
	}
	return lines
}

func checkLines(checks []sqlgen.Check) []string {
	lines := make([]string, len(checks))
	for i, chk := range checks {
		lines[i] = fmt.Sprintf("%s (%s)", fkLabel(chk.Name), chk.Expr)
	}
	return lines
}

func renderColumn(c Column) string {
	typ := c.Type
	switch {
	case typ == RawType:
		typ = c.NativeType
	case c.Length > 0:
		typ = fmt.Sprintf("%s(%d)", typ, c.Length)
	case c.Precision > 0:
		typ = fmt.Sprintf("%s(%d,%d)", typ, c.Precision, c.Scale)
	}

	var flags []string
	if c.PrimaryKey {
		flags = append(flags, "PK")
	}
	if c.AutoIncrement {
		flags = append(flags, "AUTO_INCREMENT")
	}
	if c.Nullable != nil && !*c.Nullable {
		flags = append(flags, "NOT NULL")
	}
	if c.Unique {
		flags = append(flags, "UNIQUE")
	}
	if c.Default != nil {
		def := c.Default.Expr
		if def == "" {
			def = fmt.Sprintf("%v", c.Default.Literal)
		}
		flags = append(flags, "DEFAULT "+def)
	}

	line := fmt.Sprintf("%-16s %s", c.Name, typ)
	if len(flags) > 0 {
		line += "  " + strings.Join(flags, " ")
	}
	return line
}

func renderIndex(idx Index) string {
	unique := ""
	if idx.Unique {
		unique = " UNIQUE"
	}
	line := fmt.Sprintf("%s (%s)%s", idx.Name, strings.Join(idx.Columns, ", "), unique)
	if idx.Method != "" {
		line += " " + idx.Method
	}
	if idx.Partial != "" {
		line += " WHERE " + idx.Partial
	}
	return line
}

func fkLabel(name string) string {
	if name == "" {
		return "(unnamed)"
	}
	return name
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- tree-drawing helpers ---

func isLastIndex(i, n int) bool { return i == n-1 }

func treeConnector(isLast bool) string {
	if isLast {
		return "└── "
	}
	return "├── "
}

// childPrefix computes the prefix for a node's children: continuation bars
// (│) under every ancestor that still has siblings below it, blank
// indentation under one that doesn't.
func childPrefix(prefix string, isLast bool) string {
	if isLast {
		return prefix + "    "
	}
	return prefix + "│   "
}

func writeLine(b *strings.Builder, prefix string, isLast bool, label string) {
	fmt.Fprintf(b, "%s%s%s\n", prefix, treeConnector(isLast), label)
}
