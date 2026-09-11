package scanner_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/AbdelrahmanDwedar/mig/internal/scanner"
)

func newPostgresScanner(t *testing.T) (scanner.Scanner, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
	})

	sc, err := scanner.New("postgresql", db)
	if err != nil {
		t.Fatal(err)
	}
	return sc, mock
}

// --- tableNames (via Scan on an empty/erroring database) ---

func TestPostgresScanner_EmptyDatabase(t *testing.T) {
	sc, mock := newPostgresScanner(t)

	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))
	mock.ExpectQuery("FROM information_schema.views").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "view_definition"}))
	mock.ExpectQuery("FROM information_schema.triggers").
		WillReturnRows(sqlmock.NewRows([]string{"trigger_name", "event_object_table", "action_timing", "event_manipulation", "action_statement"}))
	mock.ExpectQuery("FROM pg_class").
		WillReturnRows(sqlmock.NewRows([]string{"relname"}))

	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 0 || len(schema.Views) != 0 || len(schema.Triggers) != 0 || len(schema.Sequences) != 0 {
		t.Errorf("expected an entirely empty schema, got %+v", schema)
	}
}

func TestPostgresScanner_TableNames_QueryError(t *testing.T) {
	sc, mock := newPostgresScanner(t)

	wantErr := errors.New("connection refused")
	mock.ExpectQuery("FROM information_schema.tables").WillReturnError(wantErr)

	_, err := sc.Scan(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected tableNames error to propagate unwrapped, got %v", err)
	}
}

func TestPostgresScanner_TableNames_RowScanError(t *testing.T) {
	sc, mock := newPostgresScanner(t)

	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).
			AddRow("widgets").
			RowError(0, errors.New("scan boom")))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected an error when a tableNames row fails to scan")
	}
}

// --- scanTable orchestration / per-step error wrapping ---

func TestPostgresScanner_ScanTable_PrimaryKeyColumns_Error(t *testing.T) {
	sc, mock := newPostgresScanner(t)

	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.table_constraints").
		WillReturnError(errors.New("pk query failed"))

	_, err := sc.Scan(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got != `scanning table "widgets": pk query failed` {
		t.Errorf("expected wrapped error, got %q", got)
	}
}

func TestPostgresScanner_ScanTable_Indexes_Error(t *testing.T) {
	sc, mock := newPostgresScanner(t)

	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.table_constraints").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
	mock.ExpectQuery("FROM pg_index").
		WillReturnError(errors.New("index query failed"))

	_, err := sc.Scan(context.Background())
	if err == nil || err.Error() != `scanning table "widgets": index query failed` {
		t.Errorf("expected wrapped index error, got %v", err)
	}
}

func TestPostgresScanner_ScanTable_Columns_Error(t *testing.T) {
	sc, mock := newPostgresScanner(t)

	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.table_constraints").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
	mock.ExpectQuery("FROM pg_index").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "method", "is_unique", "is_primary", "partial_pred", "columns"}))
	mock.ExpectQuery("FROM information_schema.columns").
		WillReturnError(errors.New("columns query failed"))

	_, err := sc.Scan(context.Background())
	if err == nil || err.Error() != `scanning table "widgets": columns query failed` {
		t.Errorf("expected wrapped columns error, got %v", err)
	}
}

func TestPostgresScanner_ScanTable_ForeignKeys_Error(t *testing.T) {
	sc, mock := newPostgresScanner(t)

	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.table_constraints").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
	mock.ExpectQuery("FROM pg_index").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "method", "is_unique", "is_primary", "partial_pred", "columns"}))
	mock.ExpectQuery("FROM information_schema.columns").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}))
	mock.ExpectQuery("FROM pg_constraint").
		WillReturnError(errors.New("fk query failed"))

	_, err := sc.Scan(context.Background())
	if err == nil || err.Error() != `scanning table "widgets": fk query failed` {
		t.Errorf("expected wrapped fk error, got %v", err)
	}
}

func TestPostgresScanner_ScanTable_Checks_Error(t *testing.T) {
	sc, mock := newPostgresScanner(t)

	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.table_constraints").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
	mock.ExpectQuery("FROM pg_index").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "method", "is_unique", "is_primary", "partial_pred", "columns"}))
	mock.ExpectQuery("FROM information_schema.columns").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}))
	mock.ExpectQuery("FROM pg_constraint con").
		WillReturnRows(sqlmock.NewRows([]string{"conname", "confupdtype", "confdeltype", "columns", "ref_table", "ref_columns"}))
	mock.ExpectQuery("pg_get_constraintdef").
		WillReturnError(errors.New("checks query failed"))

	_, err := sc.Scan(context.Background())
	if err == nil || err.Error() != `scanning table "widgets": checks query failed` {
		t.Errorf("expected wrapped checks error, got %v", err)
	}
}

// --- primaryKeyColumns ---

func TestPostgresScanner_CompositePrimaryKey(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	expectFullTableScan(mock, "memberships", fullTableScanOpts{
		pkRows: sqlmock.NewRows([]string{"column_name"}).AddRow("org_id").AddRow("user_id"),
		columnRows: sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}).
			AddRow("org_id", "integer", "int4", nil, nil, nil, "NO", nil).
			AddRow("user_id", "integer", "int4", nil, nil, nil, "NO", nil),
	})
	expectEmptyGlobals(mock)

	schema := mustScanWith(t, sc)
	tbl := schema.Tables["memberships"]
	pkCount := 0
	for _, c := range tbl.Columns {
		if c.PrimaryKey {
			pkCount++
		}
	}
	if pkCount != 2 {
		t.Errorf("expected 2 primary key columns, got %d", pkCount)
	}
}

// --- scanColumns ---

func TestPostgresScanner_Columns_NullableAndDefaults(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	expectFullTableScan(mock, "widgets", fullTableScanOpts{
		columnRows: sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}).
			AddRow("id", "integer", "int4", nil, nil, nil, "NO", "nextval('widgets_id_seq'::regclass)").
			AddRow("name", "character varying", "varchar", 100, nil, nil, "NO", nil).
			AddRow("status", "character varying", "varchar", 20, nil, nil, "YES", "'active'::character varying").
			AddRow("meta", "tsvector", "tsvector", nil, nil, nil, "YES", nil),
	})
	expectEmptyGlobals(mock)

	schema := mustScanWith(t, sc)
	cols := colMap(schema.Tables["widgets"])

	id := cols["id"]
	if !id.AutoIncrement {
		t.Error("expected id.AutoIncrement=true from nextval() default")
	}
	if id.Default != nil {
		t.Errorf("expected nextval() default to NOT populate Default, got %+v", id.Default)
	}
	if id.Nullable == nil || *id.Nullable {
		t.Error("expected id to be NOT NULL")
	}

	name := cols["name"]
	if name.Type != "string" || name.Length != 100 {
		t.Errorf("name: got Type=%q Length=%d, want string(100)", name.Type, name.Length)
	}

	status := cols["status"]
	if status.Nullable == nil || !*status.Nullable {
		t.Error("expected status to be nullable")
	}
	if status.Default == nil || status.Default.Expr != "'active'::character varying" {
		t.Errorf("expected literal default preserved verbatim, got %+v", status.Default)
	}
	if status.AutoIncrement {
		t.Error("expected status.AutoIncrement=false for a literal default")
	}

	meta := cols["meta"]
	if meta.Type != scanner.RawType || meta.NativeType != "tsvector" {
		t.Errorf("expected raw fallback via udt_name for unmappable type, got Type=%q NativeType=%q", meta.Type, meta.NativeType)
	}
}

func TestPostgresScanner_Columns_QueryError(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.table_constraints").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
	mock.ExpectQuery("FROM pg_index").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "method", "is_unique", "is_primary", "partial_pred", "columns"}))
	mock.ExpectQuery("FROM information_schema.columns").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}).
			AddRow("id", "integer", "int4", nil, nil, nil, "NO", nil).
			RowError(0, errors.New("mid-scan boom")))

	_, err := sc.Scan(context.Background())
	if err == nil {
		t.Error("expected a row-scan error from scanColumns to propagate")
	}
}

// --- scanIndexes ---

func TestPostgresScanner_Indexes_FoldingRules(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	expectFullTableScan(mock, "widgets", fullTableScanOpts{
		indexRows: sqlmock.NewRows([]string{"index_name", "method", "is_unique", "is_primary", "partial_pred", "columns"}).
			AddRow("widgets_pkey", "btree", true, true, nil, "{id}").
			AddRow("widgets_sku_key", "btree", true, false, nil, "{sku}").
			AddRow("widgets_code_hash_idx", "hash", true, false, nil, "{code}").
			AddRow("widgets_active_partial_idx", "btree", true, false, "is_active = true", "{email}").
			AddRow("widgets_name_desc_idx", "gin", false, false, nil, "{name,description}"),
		columnRows: sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}),
	})
	expectEmptyGlobals(mock)

	schema := mustScanWith(t, sc)
	tbl := schema.Tables["widgets"]

	if len(tbl.Indexes) != 3 {
		t.Fatalf("expected 3 displayed indexes (hash-unique, partial-unique, composite non-unique), got %d: %+v", len(tbl.Indexes), tbl.Indexes)
	}
	byName := map[string]scanner.Index{}
	for _, idx := range tbl.Indexes {
		byName[idx.Name] = idx
	}
	if _, ok := byName["widgets_pkey"]; ok {
		t.Error("primary key index should never be displayed")
	}
	if _, ok := byName["widgets_sku_key"]; ok {
		t.Error("single-column unique btree index with no partial predicate should fold into Column.Unique")
	}
	hashIdx, ok := byName["widgets_code_hash_idx"]
	if !ok || hashIdx.Method != "hash" {
		t.Errorf("expected unique non-btree index to be displayed, got %+v", byName)
	}
	partialIdx, ok := byName["widgets_active_partial_idx"]
	if !ok || partialIdx.Partial != "is_active = true" {
		t.Errorf("expected unique btree index WITH a partial predicate to be displayed with Partial set, got %+v", byName)
	}
	compositeIdx, ok := byName["widgets_name_desc_idx"]
	if !ok || len(compositeIdx.Columns) != 2 || compositeIdx.Columns[0] != "name" || compositeIdx.Columns[1] != "description" {
		t.Errorf("expected composite index with ordered columns, got %+v", byName)
	}
}

func TestPostgresScanner_Indexes_QueryError(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.table_constraints").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
	mock.ExpectQuery("FROM pg_index").WillReturnError(errors.New("boom"))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected scanIndexes query error to propagate")
	}
}

// --- scanForeignKeys ---

func TestPostgresScanner_ForeignKeys_CompositeAndActions(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	expectFullTableScan(mock, "posts", fullTableScanOpts{
		fkRows: sqlmock.NewRows([]string{"conname", "confupdtype", "confdeltype", "columns", "ref_table", "ref_columns"}).
			AddRow("fk_posts_author", int64('r'), int64('c'), "{author_org_id,author_user_id}", "memberships", "{org_id,user_id}").
			AddRow("fk_posts_category", int64('a'), int64('n'), "{category_id}", "categories", "{id}"),
		columnRows: sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}),
	})
	expectEmptyGlobals(mock)

	schema := mustScanWith(t, sc)
	fks := schema.Tables["posts"].ForeignKeys
	if len(fks) != 2 {
		t.Fatalf("expected 2 foreign keys, got %d", len(fks))
	}

	composite := fks[0]
	if len(composite.Columns) != 2 || composite.Columns[0] != "author_org_id" || composite.Columns[1] != "author_user_id" {
		t.Errorf("expected ordered composite FK columns, got %v", composite.Columns)
	}
	if len(composite.RefColumns) != 2 || composite.RefColumns[0] != "org_id" || composite.RefColumns[1] != "user_id" {
		t.Errorf("expected ordered composite FK ref columns, got %v", composite.RefColumns)
	}
	if composite.OnUpdate != "RESTRICT" || composite.OnDelete != "CASCADE" {
		t.Errorf("expected RESTRICT/CASCADE, got OnUpdate=%q OnDelete=%q", composite.OnUpdate, composite.OnDelete)
	}

	second := fks[1]
	if second.OnUpdate != "" || second.OnDelete != "SET NULL" {
		t.Errorf("expected NO ACTION('a')->\"\" and SET NULL, got OnUpdate=%q OnDelete=%q", second.OnUpdate, second.OnDelete)
	}
}

func TestPostgresScanner_ForeignKeys_NoneAndQueryError(t *testing.T) {
	t.Run("no foreign keys", func(t *testing.T) {
		sc, mock := newPostgresScanner(t)
		expectFullTableScan(mock, "widgets", fullTableScanOpts{
			columnRows: sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}),
		})
		expectEmptyGlobals(mock)

		schema := mustScanWith(t, sc)
		if len(schema.Tables["widgets"].ForeignKeys) != 0 {
			t.Error("expected no foreign keys")
		}
	})

	t.Run("query error", func(t *testing.T) {
		sc, mock := newPostgresScanner(t)
		mock.ExpectQuery("FROM information_schema.tables").
			WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("widgets"))
		mock.ExpectQuery("FROM information_schema.table_constraints").
			WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
		mock.ExpectQuery("FROM pg_index").
			WillReturnRows(sqlmock.NewRows([]string{"index_name", "method", "is_unique", "is_primary", "partial_pred", "columns"}))
		mock.ExpectQuery("FROM information_schema.columns").
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}))
		mock.ExpectQuery("FROM pg_constraint con").WillReturnError(errors.New("boom"))

		if _, err := sc.Scan(context.Background()); err == nil {
			t.Error("expected scanForeignKeys query error to propagate")
		}
	})
}

// --- scanChecks ---

func TestPostgresScanner_Checks(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	expectFullTableScan(mock, "widgets", fullTableScanOpts{
		checkRows: sqlmock.NewRows([]string{"conname", "def"}).
			AddRow("widgets_price_check", "CHECK (price > 0)").
			AddRow("widgets_qty_check", "CHECK ((qty >= 0) AND (qty < 1000))"),
		columnRows: sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}),
	})
	expectEmptyGlobals(mock)

	schema := mustScanWith(t, sc)
	checks := schema.Tables["widgets"].Checks
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(checks))
	}
	if checks[0].Expr != "price > 0" {
		t.Errorf("expected stripped CHECK() wrapper, got %q", checks[0].Expr)
	}
	if checks[1].Expr != "(qty >= 0) AND (qty < 1000)" {
		t.Errorf("expected stripped outer wrapper only, got %q", checks[1].Expr)
	}
}

func TestPostgresScanner_Checks_QueryError(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.table_constraints").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
	mock.ExpectQuery("FROM pg_index").
		WillReturnRows(sqlmock.NewRows([]string{"index_name", "method", "is_unique", "is_primary", "partial_pred", "columns"}))
	mock.ExpectQuery("FROM information_schema.columns").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"}))
	mock.ExpectQuery("FROM pg_constraint con").
		WillReturnRows(sqlmock.NewRows([]string{"conname", "confupdtype", "confdeltype", "columns", "ref_table", "ref_columns"}))
	mock.ExpectQuery("pg_get_constraintdef").WillReturnError(errors.New("boom"))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected scanChecks query error to propagate")
	}
}

// --- scanViews ---

func TestPostgresScanner_Views(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))
	mock.ExpectQuery("FROM information_schema.views").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "view_definition"}).
			AddRow("active_users", "SELECT * FROM users WHERE active").
			AddRow("legacy_view", nil))
	mock.ExpectQuery("FROM information_schema.triggers").
		WillReturnRows(sqlmock.NewRows([]string{"trigger_name", "event_object_table", "action_timing", "event_manipulation", "action_statement"}))
	mock.ExpectQuery("FROM pg_class").
		WillReturnRows(sqlmock.NewRows([]string{"relname"}))

	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if schema.Views["active_users"].Definition == "" {
		t.Error("expected view definition to be populated")
	}
	if schema.Views["legacy_view"].Definition != "" {
		t.Errorf("expected NULL view_definition to become empty string, got %q", schema.Views["legacy_view"].Definition)
	}
}

func TestPostgresScanner_Views_QueryError(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))
	mock.ExpectQuery("FROM information_schema.views").WillReturnError(errors.New("boom"))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected scanViews error to propagate, unwrapped")
	}
}

// --- scanTriggers ---

func TestPostgresScanner_Triggers_MultiEventMerge(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))
	mock.ExpectQuery("FROM information_schema.views").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "view_definition"}))
	mock.ExpectQuery("FROM information_schema.triggers").
		WillReturnRows(sqlmock.NewRows([]string{"trigger_name", "event_object_table", "action_timing", "event_manipulation", "action_statement"}).
			AddRow("trg_touch", "widgets", "BEFORE", "INSERT", "SELECT 1").
			AddRow("trg_touch", "widgets", "BEFORE", "UPDATE", "SELECT 1").
			AddRow("trg_touch", "widgets", "BEFORE", "DELETE", "SELECT 1").
			AddRow("trg_simple", "widgets", "AFTER", "INSERT", "SELECT 2"))
	mock.ExpectQuery("FROM pg_class").
		WillReturnRows(sqlmock.NewRows([]string{"relname"}))

	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	merged := schema.Triggers["trg_touch"]
	if merged.Event != "INSERT OR UPDATE OR DELETE" {
		t.Errorf("expected merged 3-event trigger, got Event=%q", merged.Event)
	}
	if merged.Table != "widgets" || merged.Timing != "BEFORE" {
		t.Errorf("expected Table/Timing to keep the first row's values, got %+v", merged)
	}
	simple := schema.Triggers["trg_simple"]
	if simple.Event != "INSERT" {
		t.Errorf("expected unmerged single-event trigger, got %+v", simple)
	}
}

func TestPostgresScanner_Triggers_QueryError(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))
	mock.ExpectQuery("FROM information_schema.views").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "view_definition"}))
	mock.ExpectQuery("FROM information_schema.triggers").WillReturnError(errors.New("boom"))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected scanTriggers error to propagate, unwrapped")
	}
}

// --- scanSequences ---

func TestPostgresScanner_Sequences_ExcludesOwnedSequences(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))
	mock.ExpectQuery("FROM information_schema.views").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "view_definition"}))
	mock.ExpectQuery("FROM information_schema.triggers").
		WillReturnRows(sqlmock.NewRows([]string{"trigger_name", "event_object_table", "action_timing", "event_manipulation", "action_statement"}))
	// All sequences are column-owned (excluded by the NOT EXISTS pg_depend
	// clause) -> the name list comes back empty, and no per-sequence
	// QueryRowContext should ever be issued (sqlmock enforces this: an
	// unexpected query fails the test).
	mock.ExpectQuery("FROM pg_class").
		WillReturnRows(sqlmock.NewRows([]string{"relname"}))

	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Sequences) != 0 {
		t.Errorf("expected no standalone sequences, got %d", len(schema.Sequences))
	}
}

func TestPostgresScanner_Sequences_Standalone(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))
	mock.ExpectQuery("FROM information_schema.views").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "view_definition"}))
	mock.ExpectQuery("FROM information_schema.triggers").
		WillReturnRows(sqlmock.NewRows([]string{"trigger_name", "event_object_table", "action_timing", "event_manipulation", "action_statement"}))
	mock.ExpectQuery("FROM pg_class").
		WillReturnRows(sqlmock.NewRows([]string{"relname"}).AddRow("report_number_seq").AddRow("invoice_number_seq"))
	mock.ExpectQuery("FROM pg_sequences").
		WithArgs("report_number_seq").
		WillReturnRows(sqlmock.NewRows([]string{"start_value", "increment_by", "min_value", "max_value"}).
			AddRow(int64(1), int64(1), int64(1), int64(9223372036854775807)))
	mock.ExpectQuery("FROM pg_sequences").
		WithArgs("invoice_number_seq").
		WillReturnRows(sqlmock.NewRows([]string{"start_value", "increment_by", "min_value", "max_value"}).
			AddRow(int64(1000), int64(5), int64(1000), int64(999999)))

	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Sequences) != 2 {
		t.Fatalf("expected 2 standalone sequences, got %d", len(schema.Sequences))
	}
	report := schema.Sequences["report_number_seq"]
	if report.Start != 1 || report.Increment != 1 {
		t.Errorf("unexpected report_number_seq: %+v", report)
	}
	invoice := schema.Sequences["invoice_number_seq"]
	if invoice.Start != 1000 || invoice.Increment != 5 || invoice.MinValue != 1000 || invoice.MaxValue != 999999 {
		t.Errorf("unexpected invoice_number_seq: %+v", invoice)
	}
}

func TestPostgresScanner_Sequences_NameListQueryError(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))
	mock.ExpectQuery("FROM information_schema.views").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "view_definition"}))
	mock.ExpectQuery("FROM information_schema.triggers").
		WillReturnRows(sqlmock.NewRows([]string{"trigger_name", "event_object_table", "action_timing", "event_manipulation", "action_statement"}))
	mock.ExpectQuery("FROM pg_class").WillReturnError(errors.New("boom"))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected scanSequences name-list query error to propagate")
	}
}

func TestPostgresScanner_Sequences_PerSequenceQueryError(t *testing.T) {
	sc, mock := newPostgresScanner(t)
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}))
	mock.ExpectQuery("FROM information_schema.views").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "view_definition"}))
	mock.ExpectQuery("FROM information_schema.triggers").
		WillReturnRows(sqlmock.NewRows([]string{"trigger_name", "event_object_table", "action_timing", "event_manipulation", "action_statement"}))
	mock.ExpectQuery("FROM pg_class").
		WillReturnRows(sqlmock.NewRows([]string{"relname"}).AddRow("vanished_seq"))
	mock.ExpectQuery("FROM pg_sequences").
		WithArgs("vanished_seq").
		WillReturnError(sql.ErrNoRows)

	_, err := sc.Scan(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected wrapped sql.ErrNoRows, got %v", err)
	}
	if want := `scanning sequence "vanished_seq": sql: no rows in result set`; err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

// --- helpers shared by table-scoped tests ---

type fullTableScanOpts struct {
	pkRows     *sqlmock.Rows
	indexRows  *sqlmock.Rows
	columnRows *sqlmock.Rows
	fkRows     *sqlmock.Rows
	checkRows  *sqlmock.Rows
}

// expectFullTableScan wires up sqlmock expectations for one table's full
// scanTable() call sequence: primaryKeyColumns -> scanIndexes -> scanColumns
// -> scanForeignKeys -> scanChecks (Postgres's actual order).
func expectFullTableScan(mock sqlmock.Sqlmock, table string, opts fullTableScanOpts) {
	mock.ExpectQuery("FROM information_schema.tables").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow(table))

	pk := opts.pkRows
	if pk == nil {
		pk = sqlmock.NewRows([]string{"column_name"})
	}
	mock.ExpectQuery("FROM information_schema.table_constraints").WillReturnRows(pk)

	idx := opts.indexRows
	if idx == nil {
		idx = sqlmock.NewRows([]string{"index_name", "method", "is_unique", "is_primary", "partial_pred", "columns"})
	}
	mock.ExpectQuery("FROM pg_index").WillReturnRows(idx)

	cols := opts.columnRows
	if cols == nil {
		cols = sqlmock.NewRows([]string{"column_name", "data_type", "udt_name", "character_maximum_length", "numeric_precision", "numeric_scale", "is_nullable", "column_default"})
	}
	mock.ExpectQuery("FROM information_schema.columns").WillReturnRows(cols)

	fk := opts.fkRows
	if fk == nil {
		fk = sqlmock.NewRows([]string{"conname", "confupdtype", "confdeltype", "columns", "ref_table", "ref_columns"})
	}
	mock.ExpectQuery("FROM pg_constraint con").WillReturnRows(fk)

	checks := opts.checkRows
	if checks == nil {
		checks = sqlmock.NewRows([]string{"conname", "def"})
	}
	mock.ExpectQuery("pg_get_constraintdef").WillReturnRows(checks)
}

// expectEmptyGlobals wires up empty views/triggers/sequences queries for
// tests that only care about a single table.
func expectEmptyGlobals(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("FROM information_schema.views").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "view_definition"}))
	mock.ExpectQuery("FROM information_schema.triggers").
		WillReturnRows(sqlmock.NewRows([]string{"trigger_name", "event_object_table", "action_timing", "event_manipulation", "action_statement"}))
	mock.ExpectQuery("FROM pg_class").
		WillReturnRows(sqlmock.NewRows([]string{"relname"}))
}

func mustScanWith(t *testing.T, sc scanner.Scanner) *scanner.Schema {
	t.Helper()
	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func colMap(tbl *scanner.Table) map[string]scanner.Column {
	m := map[string]scanner.Column{}
	for _, c := range tbl.Columns {
		m[c.Name] = c
	}
	return m
}
