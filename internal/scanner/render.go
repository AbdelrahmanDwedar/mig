package scanner

import (
	"fmt"
	"sort"
	"strings"
)

// RenderText renders a Schema as a human-readable tree, the default (non
// --json) output of `mig inspect`.
func RenderText(schema *Schema) string {
	var b strings.Builder

	tableNames := sortedKeys(schema.Tables)
	fmt.Fprintf(&b, "Tables (%d)\n", len(tableNames))
	for _, name := range tableNames {
		renderTable(&b, schema.Tables[name])
	}

	if len(schema.Views) > 0 {
		names := sortedKeys(schema.Views)
		fmt.Fprintf(&b, "\nViews (%d)\n", len(names))
		for _, name := range names {
			v := schema.Views[name]
			if v.Definition != "" {
				fmt.Fprintf(&b, "  %s — %s\n", v.Name, oneLine(v.Definition))
			} else {
				fmt.Fprintf(&b, "  %s\n", v.Name)
			}
		}
	}

	if len(schema.Triggers) > 0 {
		names := sortedKeys(schema.Triggers)
		fmt.Fprintf(&b, "\nTriggers (%d)\n", len(names))
		for _, name := range names {
			t := schema.Triggers[name]
			fmt.Fprintf(&b, "  %s (%s, %s %s)\n", t.Name, t.Table, t.Timing, t.Event)
		}
	}

	if len(schema.Sequences) > 0 {
		names := sortedKeys(schema.Sequences)
		fmt.Fprintf(&b, "\nSequences (%d)\n", len(names))
		for _, name := range names {
			s := schema.Sequences[name]
			fmt.Fprintf(&b, "  %s (start %d, increment %d)\n", s.Name, s.Start, s.Increment)
		}
	}

	return b.String()
}

func renderTable(b *strings.Builder, t *Table) {
	fmt.Fprintf(b, "  %s\n", t.Name)

	fmt.Fprintf(b, "    Columns\n")
	for _, c := range t.Columns {
		fmt.Fprintf(b, "      %s\n", renderColumn(c))
	}

	fmt.Fprintf(b, "    Indexes\n")
	if len(t.Indexes) == 0 {
		fmt.Fprintf(b, "      (none)\n")
	}
	for _, idx := range t.Indexes {
		fmt.Fprintf(b, "      %s\n", renderIndex(idx))
	}

	fmt.Fprintf(b, "    Foreign Keys\n")
	if len(t.ForeignKeys) == 0 {
		fmt.Fprintf(b, "      (none)\n")
	}
	for _, fk := range t.ForeignKeys {
		fmt.Fprintf(b, "      %s (%s) -> %s (%s)", fkLabel(fk.Name), strings.Join(fk.Columns, ", "), fk.RefTable, strings.Join(fk.RefColumns, ", "))
		if fk.OnDelete != "" {
			fmt.Fprintf(b, " ON DELETE %s", fk.OnDelete)
		}
		if fk.OnUpdate != "" {
			fmt.Fprintf(b, " ON UPDATE %s", fk.OnUpdate)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(b, "    Checks\n")
	if len(t.Checks) == 0 {
		fmt.Fprintf(b, "      (none)\n")
	}
	for _, chk := range t.Checks {
		fmt.Fprintf(b, "      %s (%s)\n", fkLabel(chk.Name), chk.Expr)
	}
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
