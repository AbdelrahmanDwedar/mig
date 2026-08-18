package template_test

import (
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/sqlgen"
	"github.com/AbdelrahmanDwedar/mig/internal/template"
)

func TestParseColumn_Modifiers(t *testing.T) {
	cases := []struct {
		name  string
		field string
		want  sqlgen.Column
	}{
		{
			name:  "bare type",
			field: "bio:text",
			want:  sqlgen.Column{Name: "bio", Type: "text"},
		},
		{
			name:  "string with length",
			field: "email:string(255)",
			want:  sqlgen.Column{Name: "email", Type: "string", Length: 255},
		},
		{
			name:  "decimal with precision and scale",
			field: "price:decimal(10,2)",
			want:  sqlgen.Column{Name: "price", Type: "decimal", Precision: 10, Scale: 2},
		},
		{
			name:  "pk and auto",
			field: "id:bigint:pk:auto",
			want:  sqlgen.Column{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true},
		},
		{
			name:  "unique",
			field: "email:string(255):unique",
			want:  sqlgen.Column{Name: "email", Type: "string", Length: 255, Unique: true},
		},
		{
			name:  "null",
			field: "bio:text:null",
			want:  sqlgen.Column{Name: "bio", Type: "text", Nullable: boolPtr(true)},
		},
		{
			name:  "notnull",
			field: "bio:text:notnull",
			want:  sqlgen.Column{Name: "bio", Type: "text", Nullable: boolPtr(false)},
		},
		{
			name:  "default numeric literal",
			field: "price:decimal(10,2):default=0",
			want: sqlgen.Column{Name: "price", Type: "decimal", Precision: 10, Scale: 2,
				Default: &sqlgen.Default{Literal: float64(0)}},
		},
		{
			name:  "default bool literal",
			field: "active:boolean:default=true",
			want:  sqlgen.Column{Name: "active", Type: "boolean", Default: &sqlgen.Default{Literal: true}},
		},
		{
			name:  "default string literal",
			field: "plan:string(50):default=free",
			want: sqlgen.Column{Name: "plan", Type: "string", Length: 50,
				Default: &sqlgen.Default{Literal: "free"}},
		},
		{
			name:  "default expression",
			field: "created_at:timestamp:defaultexpr=NOW()",
			want:  sqlgen.Column{Name: "created_at", Type: "timestamp", Default: &sqlgen.Default{Expr: "NOW()"}},
		},
		{
			name:  "combined modifiers",
			field: "id:bigint:pk:auto:notnull",
			want:  sqlgen.Column{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, Nullable: boolPtr(false)},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := template.ParseColumn(c.field)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertColumnEqual(t, got, c.want)
		})
	}
}

func TestParseColumn_Errors(t *testing.T) {
	cases := []string{
		"",
		"noType",
		":string",
		"name:unknown_type",
		"name:string:badmodifier",
		"name:decimal(abc,2)",
	}
	for _, field := range cases {
		if _, err := template.ParseColumn(field); err == nil {
			t.Errorf("ParseColumn(%q): expected error, got nil", field)
		}
	}
}

func TestParseColumns(t *testing.T) {
	cols, err := template.ParseColumns("id:bigint:pk:auto, email:string(255):unique, price:decimal(10,2)")
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	if cols[0].Name != "id" || cols[1].Name != "email" || cols[2].Name != "price" {
		t.Fatalf("unexpected column order/names: %+v", cols)
	}
}

func TestParseColumns_Empty(t *testing.T) {
	cols, err := template.ParseColumns("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 0 {
		t.Fatalf("expected 0 columns for empty spec, got %d", len(cols))
	}
}

func TestParseIndex(t *testing.T) {
	idx, err := template.ParseIndex("email,name", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Name != "idx_email_name" {
		t.Errorf("expected auto-generated name idx_email_name, got %q", idx.Name)
	}
	if !idx.Unique {
		t.Error("expected unique=true")
	}
	if len(idx.Columns) != 2 || idx.Columns[0] != "email" || idx.Columns[1] != "name" {
		t.Errorf("unexpected columns: %+v", idx.Columns)
	}

	named, err := template.ParseIndex("email", "idx_custom", false)
	if err != nil {
		t.Fatal(err)
	}
	if named.Name != "idx_custom" {
		t.Errorf("expected explicit name to win, got %q", named.Name)
	}
}

func TestParseIndex_Errors(t *testing.T) {
	if _, err := template.ParseIndex("", "", false); err == nil {
		t.Fatal("expected error for empty columns")
	}
	if _, err := template.ParseIndex("email,,name", "", false); err == nil {
		t.Fatal("expected error for empty column in list")
	}
}

func TestParseForeignKey(t *testing.T) {
	fk, err := template.ParseForeignKey([]string{"user_id"}, "users(id)", "cascade", "restrict", "")
	if err != nil {
		t.Fatal(err)
	}
	if fk.RefTable != "users" || len(fk.RefColumns) != 1 || fk.RefColumns[0] != "id" {
		t.Errorf("unexpected FK ref: %+v", fk)
	}
	if fk.Name != "fk_user_id_users" {
		t.Errorf("expected auto-generated name, got %q", fk.Name)
	}
	if fk.OnDelete != "cascade" || fk.OnUpdate != "restrict" {
		t.Errorf("unexpected on_delete/on_update: %+v", fk)
	}

	composite, err := template.ParseForeignKey([]string{"a", "b"}, "other(x, y)", "", "", "fk_custom")
	if err != nil {
		t.Fatal(err)
	}
	if len(composite.RefColumns) != 2 || composite.RefColumns[0] != "x" || composite.RefColumns[1] != "y" {
		t.Errorf("unexpected composite ref columns: %+v", composite.RefColumns)
	}
	if composite.Name != "fk_custom" {
		t.Errorf("expected explicit name to win, got %q", composite.Name)
	}
}

func TestParseForeignKey_Errors(t *testing.T) {
	if _, err := template.ParseForeignKey(nil, "users(id)", "", "", ""); err == nil {
		t.Fatal("expected error for no columns")
	}
	if _, err := template.ParseForeignKey([]string{"user_id"}, "malformed", "", "", ""); err == nil {
		t.Fatal("expected error for malformed references")
	}
	if _, err := template.ParseForeignKey([]string{"user_id"}, "", "", "", ""); err == nil {
		t.Fatal("expected error for empty references")
	}
}

func boolPtr(b bool) *bool { return &b }

func assertColumnEqual(t *testing.T, got, want sqlgen.Column) {
	t.Helper()
	if got.Name != want.Name || got.Type != want.Type || got.Length != want.Length ||
		got.Precision != want.Precision || got.Scale != want.Scale ||
		got.PrimaryKey != want.PrimaryKey || got.Unique != want.Unique || got.AutoIncrement != want.AutoIncrement {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if (got.Nullable == nil) != (want.Nullable == nil) {
		t.Fatalf("nullable mismatch: got %+v, want %+v", got, want)
	}
	if got.Nullable != nil && want.Nullable != nil && *got.Nullable != *want.Nullable {
		t.Fatalf("nullable value mismatch: got %v, want %v", *got.Nullable, *want.Nullable)
	}
	if (got.Default == nil) != (want.Default == nil) {
		t.Fatalf("default mismatch: got %+v, want %+v", got.Default, want.Default)
	}
	if got.Default != nil && want.Default != nil {
		if got.Default.Expr != want.Default.Expr {
			t.Fatalf("default expr mismatch: got %q, want %q", got.Default.Expr, want.Default.Expr)
		}
		if got.Default.Literal != want.Default.Literal {
			t.Fatalf("default literal mismatch: got %#v, want %#v", got.Default.Literal, want.Default.Literal)
		}
	}
}
