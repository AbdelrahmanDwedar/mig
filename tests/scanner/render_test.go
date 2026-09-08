package scanner_test

import (
	"strings"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/scanner"
	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
)

// TestRenderText_MentionsEveryObject builds a Schema with one of every
// object kind (a composite-PK table, a dialect-specific index, a view, a
// trigger, a sequence) and asserts RenderText doesn't panic and doesn't
// silently drop any of them from the rendered output.
func TestRenderText_MentionsEveryObject(t *testing.T) {
	nullable := false
	schema := &scanner.Schema{
		Tables: map[string]*scanner.Table{
			"memberships": {
				Name: "memberships",
				Columns: []scanner.Column{
					{Column: sqlgen.Column{Name: "org_id", Type: "integer", PrimaryKey: true, Nullable: &nullable}},
					{Column: sqlgen.Column{Name: "user_id", Type: "integer", PrimaryKey: true, Nullable: &nullable}},
					{Column: sqlgen.Column{Name: "weird_col", Type: scanner.RawType}, NativeType: "HSTORE"},
				},
				Indexes: []scanner.Index{
					{Index: sqlgen.Index{Name: "idx_memberships_gin", Columns: []string{"weird_col"}}, Method: "gin", Partial: "org_id > 0"},
				},
				ForeignKeys: []sqlgen.ForeignKey{
					{Name: "fk_org", Columns: []string{"org_id"}, RefTable: "orgs", RefColumns: []string{"id"}, OnDelete: "CASCADE"},
				},
				Checks: []sqlgen.Check{
					{Name: "chk_positive", Expr: "org_id > 0"},
				},
			},
		},
		Views: map[string]*scanner.View{
			"active_memberships": {Name: "active_memberships", Definition: "SELECT * FROM memberships"},
		},
		Triggers: map[string]*scanner.Trigger{
			"trg_touch": {Name: "trg_touch", Table: "memberships", Timing: "AFTER", Event: "UPDATE"},
		},
		Sequences: map[string]*scanner.Sequence{
			"custom_seq": {Name: "custom_seq", Start: 1, Increment: 1},
		},
	}

	out := scanner.RenderText(schema)

	for _, want := range []string{
		"memberships", "org_id", "user_id", "weird_col", "HSTORE",
		"idx_memberships_gin", "gin", "org_id > 0",
		"fk_org", "orgs", "CASCADE",
		"chk_positive",
		"active_memberships",
		"trg_touch",
		"custom_seq",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected rendered output to mention %q, got:\n%s", want, out)
		}
	}
}

func TestRenderText_EmptySchema(t *testing.T) {
	schema := &scanner.Schema{Tables: map[string]*scanner.Table{}}
	out := scanner.RenderText(schema)
	if !strings.Contains(out, "Tables (0)") {
		t.Errorf("expected empty schema to render \"Tables (0)\", got:\n%s", out)
	}
}
