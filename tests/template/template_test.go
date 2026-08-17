package template_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
	"github.com/AbdelrahmanDwedar/mig/internal/template"
)

func TestBuildOp_UnknownTemplate(t *testing.T) {
	if _, err := template.BuildOp("not_a_real_op", template.Flags{Table: "users"}); err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestBuildOp_RequiredFlags(t *testing.T) {
	cases := []struct {
		kind  string
		flags template.Flags
	}{
		{"create_table", template.Flags{}},                               // missing --table
		{"create_table", template.Flags{Table: "users"}},                 // missing --columns
		{"add_column", template.Flags{Table: "users"}},                   // missing --columns
		{"drop_column", template.Flags{Table: "users"}},                  // missing --column
		{"rename_column", template.Flags{Table: "users", Column: "old"}}, // missing --to
		{"rename_table", template.Flags{Table: "users"}},                 // missing --to
		{"add_index", template.Flags{Table: "users"}},                    // missing --index
		{"drop_index", template.Flags{Table: "users"}},                   // missing --index-name
		{"add_foreign_key", template.Flags{Table: "posts"}},              // missing --fk-column/--fk-references
		{"drop_foreign_key", template.Flags{Table: "posts"}},             // missing --fk-name
		{"drop_table", template.Flags{}},                                 // missing --table
	}
	for _, c := range cases {
		if _, err := template.BuildOp(c.kind, c.flags); err == nil {
			t.Errorf("BuildOp(%q, %+v): expected error for missing required flag, got nil", c.kind, c.flags)
		}
	}
}

func TestBuildOp_HappyPaths(t *testing.T) {
	t.Run("create_table", func(t *testing.T) {
		op, err := template.BuildOp("create_table", template.Flags{
			Table: "users", Columns: "id:bigint:pk:auto,email:string(255):unique", IfNotExists: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		ct, ok := op.Up.(sqlgen.CreateTableOp)
		if !ok || ct.Table != "users" || !ct.IfNotExists || len(ct.Columns) != 2 {
			t.Fatalf("unexpected op: %+v", op)
		}
	})

	t.Run("add_column", func(t *testing.T) {
		op, err := template.BuildOp("add_column", template.Flags{Table: "users", Columns: "plan:string(50):default=free"})
		if err != nil {
			t.Fatal(err)
		}
		ac, ok := op.Up.(sqlgen.AddColumnOp)
		if !ok || ac.Table != "users" || ac.Column.Name != "plan" {
			t.Fatalf("unexpected op: %+v", op)
		}
	})

	t.Run("add_index", func(t *testing.T) {
		op, err := template.BuildOp("add_index", template.Flags{Table: "users", IndexColumns: "email", IndexUnique: true})
		if err != nil {
			t.Fatal(err)
		}
		ai, ok := op.Up.(sqlgen.AddIndexOp)
		if !ok || ai.Table != "users" || !ai.Index.Unique {
			t.Fatalf("unexpected op: %+v", op)
		}
	})

	t.Run("add_foreign_key", func(t *testing.T) {
		op, err := template.BuildOp("add_foreign_key", template.Flags{
			Table: "posts", FKColumns: []string{"user_id"}, FKReferences: "users(id)", FKOnDelete: "cascade",
		})
		if err != nil {
			t.Fatal(err)
		}
		fk, ok := op.Up.(sqlgen.AddForeignKeyOp)
		if !ok || fk.Table != "posts" || fk.ForeignKey.RefTable != "users" {
			t.Fatalf("unexpected op: %+v", op)
		}
	})
}

func TestDown_MechanicalInverse(t *testing.T) {
	cases := []struct {
		name     string
		up       template.Op
		wantKind string
		check    func(t *testing.T, down template.Op)
	}{
		{
			name:     "create_table -> drop_table",
			up:       template.Op{Kind: "create_table", Up: sqlgen.CreateTableOp{Table: "users"}},
			wantKind: "drop_table",
			check: func(t *testing.T, down template.Op) {
				if down.Up.(sqlgen.DropTableOp).Table != "users" {
					t.Fatalf("unexpected down: %+v", down)
				}
			},
		},
		{
			name:     "add_column -> drop_column",
			up:       template.Op{Kind: "add_column", Up: sqlgen.AddColumnOp{Table: "users", Column: sqlgen.Column{Name: "plan"}}},
			wantKind: "drop_column",
			check: func(t *testing.T, down template.Op) {
				d := down.Up.(sqlgen.DropColumnOp)
				if d.Table != "users" || d.Column != "plan" {
					t.Fatalf("unexpected down: %+v", down)
				}
			},
		},
		{
			name:     "add_index -> drop_index",
			up:       template.Op{Kind: "add_index", Up: sqlgen.AddIndexOp{Table: "users", Index: sqlgen.Index{Name: "idx_email"}}},
			wantKind: "drop_index",
			check: func(t *testing.T, down template.Op) {
				d := down.Up.(sqlgen.DropIndexOp)
				if d.Table != "users" || d.Name != "idx_email" {
					t.Fatalf("unexpected down: %+v", down)
				}
			},
		},
		{
			name:     "add_foreign_key -> drop_foreign_key",
			up:       template.Op{Kind: "add_foreign_key", Up: sqlgen.AddForeignKeyOp{Table: "posts", ForeignKey: sqlgen.ForeignKey{Name: "fk_posts_user"}}},
			wantKind: "drop_foreign_key",
			check: func(t *testing.T, down template.Op) {
				d := down.Up.(sqlgen.DropForeignKeyOp)
				if d.Table != "posts" || d.Name != "fk_posts_user" {
					t.Fatalf("unexpected down: %+v", down)
				}
			},
		},
		{
			name:     "rename_table swaps From/To",
			up:       template.Op{Kind: "rename_table", Up: sqlgen.RenameTableOp{From: "old", To: "new"}},
			wantKind: "rename_table",
			check: func(t *testing.T, down template.Op) {
				d := down.Up.(sqlgen.RenameTableOp)
				if d.From != "new" || d.To != "old" {
					t.Fatalf("unexpected down: %+v", down)
				}
			},
		},
		{
			name:     "rename_column swaps From/To",
			up:       template.Op{Kind: "rename_column", Up: sqlgen.RenameColumnOp{Table: "users", From: "old", To: "new"}},
			wantKind: "rename_column",
			check: func(t *testing.T, down template.Op) {
				d := down.Up.(sqlgen.RenameColumnOp)
				if d.Table != "users" || d.From != "new" || d.To != "old" {
					t.Fatalf("unexpected down: %+v", down)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			down := template.Down(c.up)
			if down.Kind != c.wantKind {
				t.Fatalf("expected down kind %q, got %q", c.wantKind, down.Kind)
			}
			c.check(t, down)
		})
	}
}

func TestDown_DestructiveOpsGetTODOStub(t *testing.T) {
	destructive := []template.Op{
		{Kind: "drop_table", Up: sqlgen.DropTableOp{Table: "users"}},
		{Kind: "drop_column", Up: sqlgen.DropColumnOp{Table: "users", Column: "plan"}},
		{Kind: "drop_index", Up: sqlgen.DropIndexOp{Table: "users", Name: "idx_email"}},
		{Kind: "drop_foreign_key", Up: sqlgen.DropForeignKeyOp{Table: "posts", Name: "fk_posts_user"}},
	}
	for _, up := range destructive {
		down := template.Down(up)
		if down.Kind != "sql" {
			t.Fatalf("%s: expected TODO stub kind \"sql\", got %q", up.Kind, down.Kind)
		}
		q := down.Up.(sqlgen.SQLOp).Query
		if !strings.Contains(q, "TODO") {
			t.Fatalf("%s: expected TODO stub query, got %q", up.Kind, q)
		}
	}
}

func TestRenderSQL_MatchesDialectDirectly(t *testing.T) {
	for _, driver := range []string{"postgresql", "mysql", "sqlite"} {
		t.Run(driver, func(t *testing.T) {
			dialect, err := sqlgen.New(driver)
			if err != nil {
				t.Fatal(err)
			}

			op, err := template.BuildOp("create_table", template.Flags{
				Table: "users", Columns: "id:bigint:pk:auto,email:string(255):unique",
			})
			if err != nil {
				t.Fatal(err)
			}

			content, err := template.RenderSQL(dialect, op)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(content, "-- +migrate Up") || !strings.Contains(content, "-- +migrate Down") {
				t.Fatalf("missing migrate markers:\n%s", content)
			}

			wantUpStmts, err := dialect.CreateTable(op.Up.(sqlgen.CreateTableOp))
			if err != nil {
				t.Fatal(err)
			}
			for _, stmt := range wantUpStmts {
				if !strings.Contains(content, stmt) {
					t.Fatalf("expected up section to contain dialect-generated statement %q:\n%s", stmt, content)
				}
			}

			wantDownStmts, err := dialect.DropTable(sqlgen.DropTableOp{Table: "users"})
			if err != nil {
				t.Fatal(err)
			}
			for _, stmt := range wantDownStmts {
				if !strings.Contains(content, stmt) {
					t.Fatalf("expected down section to contain dialect-generated statement %q:\n%s", stmt, content)
				}
			}
		})
	}
}

func TestRenderSQL_SurfacesDialectUnsupportedError(t *testing.T) {
	dialect, err := sqlgen.New("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	op, err := template.BuildOp("add_foreign_key", template.Flags{
		Table: "posts", FKColumns: []string{"user_id"}, FKReferences: "users(id)",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := template.RenderSQL(dialect, op); err == nil {
		t.Fatal("expected sqlite's real unsupported-op error, got nil")
	}
}

func TestRenderJSON_RoundTripsThroughBuildStatement(t *testing.T) {
	op, err := template.BuildOp("create_table", template.Flags{
		Table: "users", Columns: "id:bigint:pk:auto,email:string(255):unique",
	})
	if err != nil {
		t.Fatal(err)
	}

	content, err := template.RenderJSON(op)
	if err != nil {
		t.Fatal(err)
	}

	var migration sqlgen.Migration
	if err := json.Unmarshal([]byte(content), &migration); err != nil {
		t.Fatalf("RenderJSON produced invalid JSON: %v\n%s", err, content)
	}
	if len(migration.Up) != 1 || len(migration.Down) != 1 {
		t.Fatalf("expected exactly one up op and one down op, got up=%d down=%d", len(migration.Up), len(migration.Down))
	}

	for _, driver := range []string{"postgresql", "mysql", "sqlite"} {
		dialect, err := sqlgen.New(driver)
		if err != nil {
			t.Fatal(err)
		}

		jsonUpStmts, err := sqlgen.BuildStatement(dialect, migration.Up[0])
		if err != nil {
			t.Fatalf("%s: BuildStatement(up) failed: %v", driver, err)
		}
		directUpStmts, err := dialect.CreateTable(op.Up.(sqlgen.CreateTableOp))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(jsonUpStmts, ";") != strings.Join(directUpStmts, ";") {
			t.Errorf("%s: JSON-path up statements differ from direct RenderSQL statements:\njson:   %v\ndirect: %v", driver, jsonUpStmts, directUpStmts)
		}

		jsonDownStmts, err := sqlgen.BuildStatement(dialect, migration.Down[0])
		if err != nil {
			t.Fatalf("%s: BuildStatement(down) failed: %v", driver, err)
		}
		directDownStmts, err := dialect.DropTable(sqlgen.DropTableOp{Table: "users"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(jsonDownStmts, ";") != strings.Join(directDownStmts, ";") {
			t.Errorf("%s: JSON-path down statements differ from direct dialect call:\njson:   %v\ndirect: %v", driver, jsonDownStmts, directDownStmts)
		}
	}
}
