package scanner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/AbdelrahmanDwedar/mig/internal/scanner"
)

func newMySQLScanner(t *testing.T) (scanner.Scanner, sqlmock.Sqlmock) {
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

	sc, err := scanner.New("mysql", db)
	if err != nil {
		t.Fatal(err)
	}
	return sc, mock
}

func TestMySQLScanner_EmptyDatabase(t *testing.T) {
	sc, mock := newMySQLScanner(t)

	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}))
	mock.ExpectQuery("FROM information_schema.VIEWS").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "VIEW_DEFINITION"}))
	mock.ExpectQuery("FROM information_schema.TRIGGERS").
		WillReturnRows(sqlmock.NewRows([]string{"TRIGGER_NAME", "EVENT_OBJECT_TABLE", "ACTION_TIMING", "EVENT_MANIPULATION", "ACTION_STATEMENT"}))

	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Tables) != 0 || len(schema.Views) != 0 || len(schema.Triggers) != 0 {
		t.Errorf("expected an empty schema, got %+v", schema)
	}
	if len(schema.Sequences) != 0 {
		t.Error("expected Sequences to always be empty for MySQL (no sequence concept)")
	}
}

func TestMySQLScanner_TableNames_QueryError(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	wantErr := errors.New("connection refused")
	mock.ExpectQuery("FROM information_schema.TABLES").WillReturnError(wantErr)

	_, err := sc.Scan(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected tableNames error to propagate unwrapped, got %v", err)
	}
}

func TestMySQLScanner_TableNames_RowScanError(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).
			AddRow("widgets").
			RowError(0, errors.New("scan boom")))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected an error when a tableNames row fails to scan")
	}
}

// --- scanTable orchestration (indexes BEFORE columns, unlike Postgres) ---

func TestMySQLScanner_ScanTable_Indexes_Error(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.STATISTICS").WillReturnError(errors.New("index query failed"))

	_, err := sc.Scan(context.Background())
	if err == nil || err.Error() != `scanning table "widgets": index query failed` {
		t.Errorf("expected wrapped index error, got %v", err)
	}
}

func TestMySQLScanner_ScanTable_Columns_Error(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.STATISTICS").
		WillReturnRows(sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"}))
	mock.ExpectQuery("FROM information_schema.COLUMNS").WillReturnError(errors.New("columns query failed"))

	_, err := sc.Scan(context.Background())
	if err == nil || err.Error() != `scanning table "widgets": columns query failed` {
		t.Errorf("expected wrapped columns error, got %v", err)
	}
}

func TestMySQLScanner_ScanTable_ForeignKeys_Error(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.STATISTICS").
		WillReturnRows(sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"}))
	mock.ExpectQuery("FROM information_schema.COLUMNS").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "COLUMN_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA", "COLUMN_KEY"}))
	mock.ExpectQuery("FROM information_schema.KEY_COLUMN_USAGE").WillReturnError(errors.New("fk query failed"))

	_, err := sc.Scan(context.Background())
	if err == nil || err.Error() != `scanning table "widgets": fk query failed` {
		t.Errorf("expected wrapped fk error, got %v", err)
	}
}

// --- mysqlNativeType is covered directly (white-box) in
// internal/scanner/native_types_test.go. These tests cover the same
// branches end-to-end through scanColumns. ---

func TestMySQLScanner_Columns_TypesDefaultsAndKeys(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	expectMySQLFullTableScan(t, mock, "widgets", mysqlTableScanOpts{
		columnRows: sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "COLUMN_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA", "COLUMN_KEY"}).
			AddRow("id", "int", "int(11)", nil, nil, nil, "NO", nil, "auto_increment", "PRI").
			AddRow("sku", "char", "char(36)", 36, nil, nil, "NO", nil, "", "UNI").
			AddRow("status", "enum", "enum('a','b')", nil, nil, nil, "YES", "a", "", "").
			AddRow("created_at", "datetime", "datetime", nil, nil, nil, "NO", "CURRENT_TIMESTAMP", "DEFAULT_GENERATED", ""),
	})

	schema := mustScanWith(t, sc)
	cols := colMap(schema.Tables["widgets"])

	id := cols["id"]
	if !id.PrimaryKey {
		t.Error("expected COLUMN_KEY=PRI to set PrimaryKey")
	}
	if !id.AutoIncrement {
		t.Error("expected EXTRA containing auto_increment to set AutoIncrement")
	}

	sku := cols["sku"]
	if sku.PrimaryKey {
		t.Error("expected COLUMN_KEY=UNI to NOT set PrimaryKey")
	}
	if sku.Type == scanner.RawType {
		// char(36) maps through mysqlNativeType -> CHAR(36) -> AbstractType,
		// so it must resolve to a real abstract type, not the raw fallback.
		t.Errorf("expected char(36) to resolve to a mapped type, got Type=%q NativeType=%q", sku.Type, sku.NativeType)
	}

	status := cols["status"]
	if status.Type != scanner.RawType || status.NativeType != "enum('a','b')" {
		t.Errorf("expected ENUM to fall back to raw/COLUMN_TYPE (not DATA_TYPE), got Type=%q NativeType=%q", status.Type, status.NativeType)
	}
	if status.Default == nil || status.Default.Expr != "a" {
		t.Errorf("expected default to be preserved even for a raw-fallback type, got %+v", status.Default)
	}

	created := cols["created_at"]
	if created.AutoIncrement {
		t.Error("expected DEFAULT_GENERATED (no auto_increment substring) to NOT set AutoIncrement")
	}
	if created.Default == nil || created.Default.Expr != "CURRENT_TIMESTAMP" {
		t.Errorf("expected Default to be set independent of AutoIncrement in MySQL, got %+v", created.Default)
	}
}

func TestMySQLScanner_Columns_AutoIncrementAndDefault_NotMutuallyExclusive(t *testing.T) {
	// Contrast with Postgres: a nextval() default excludes Default and sets
	// AutoIncrement instead. MySQL has no such exclusion — EXTRA and
	// COLUMN_DEFAULT are independent columns, so both can be populated.
	sc, mock := newMySQLScanner(t)
	expectMySQLFullTableScan(t, mock, "counters", mysqlTableScanOpts{
		columnRows: sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "COLUMN_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA", "COLUMN_KEY"}).
			AddRow("n", "int", "int(11)", nil, nil, nil, "NO", "0", "STORED GENERATED auto_increment", "PRI"),
	})

	schema := mustScanWith(t, sc)
	n := colMap(schema.Tables["counters"])["n"]
	if !n.AutoIncrement {
		t.Error("expected Contains-based match on a compound EXTRA value containing auto_increment")
	}
	if n.Default == nil || n.Default.Expr != "0" {
		t.Errorf("expected Default to remain populated alongside AutoIncrement, got %+v", n.Default)
	}
}

func TestMySQLScanner_Columns_QueryError(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.STATISTICS").
		WillReturnRows(sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"}))
	mock.ExpectQuery("FROM information_schema.COLUMNS").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "COLUMN_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA", "COLUMN_KEY"}).
			AddRow("id", "int", "int(11)", nil, nil, nil, "NO", nil, "", "PRI").
			RowError(0, errors.New("mid-scan boom")))

	_, err := sc.Scan(context.Background())
	if err == nil {
		t.Error("expected a row-scan error from scanColumns to propagate")
	}
}

// --- scanIndexes ---

func TestMySQLScanner_Indexes_FoldingRules(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	expectMySQLFullTableScan(t, mock, "widgets", mysqlTableScanOpts{
		indexRows: sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"}).
			AddRow("PRIMARY", 0, "id", "BTREE").
			AddRow("sku_unique", 0, "sku", "BTREE").
			AddRow("code_fulltext", 0, "code", "FULLTEXT").
			AddRow("name_desc_idx", 1, "name", "BTREE").
			AddRow("name_desc_idx", 1, "description", "BTREE"),
	})

	schema := mustScanWith(t, sc)
	tbl := schema.Tables["widgets"]
	if len(tbl.Indexes) != 2 {
		t.Fatalf("expected 2 displayed indexes (fulltext-unique, composite non-unique); PRIMARY skipped, sku_unique folded; got %d: %+v", len(tbl.Indexes), tbl.Indexes)
	}
	byName := map[string]scanner.Index{}
	for _, idx := range tbl.Indexes {
		byName[idx.Name] = idx
	}
	if _, ok := byName["PRIMARY"]; ok {
		t.Error("PRIMARY index should always be skipped")
	}
	if _, ok := byName["sku_unique"]; ok {
		t.Error("single-column unique BTREE index should fold into Column.Unique")
	}
	if ft, ok := byName["code_fulltext"]; !ok || ft.Method != "fulltext" {
		t.Errorf("expected unique non-btree (fulltext) index to be displayed with lowercased method, got %+v", byName)
	}
	if composite, ok := byName["name_desc_idx"]; !ok || len(composite.Columns) != 2 {
		t.Errorf("expected composite index preserved with 2 columns, got %+v", byName)
	}
}

func TestMySQLScanner_Indexes_CaseInsensitiveTypeFolding(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	expectMySQLFullTableScan(t, mock, "widgets", mysqlTableScanOpts{
		indexRows: sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"}).
			AddRow("sku_unique", 0, "sku", "BTREE"),
		columnRows: sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "COLUMN_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA", "COLUMN_KEY"}).
			AddRow("sku", "varchar", "varchar(50)", 50, nil, nil, "NO", nil, "", ""),
	})

	schema := mustScanWith(t, sc)
	tbl := schema.Tables["widgets"]
	if len(tbl.Indexes) != 0 {
		t.Errorf("expected uppercase BTREE INDEX_TYPE to still fold via case-insensitive lowering, got %+v", tbl.Indexes)
	}
	sku := colMap(tbl)["sku"]
	if !sku.Unique {
		t.Error("expected sku to be marked Unique")
	}
}

func TestMySQLScanner_Indexes_NonUniqueAsInt(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	expectMySQLFullTableScan(t, mock, "widgets", mysqlTableScanOpts{
		indexRows: sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"}).
			AddRow("regular_idx", 1, "name", "BTREE"),
	})

	schema := mustScanWith(t, sc)
	tbl := schema.Tables["widgets"]
	if len(tbl.Indexes) != 1 || tbl.Indexes[0].Unique {
		t.Errorf("expected NON_UNIQUE=1 (int) to produce a non-unique index, got %+v", tbl.Indexes)
	}
}

func TestMySQLScanner_Indexes_QueryError(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.STATISTICS").WillReturnError(errors.New("boom"))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected scanIndexes query error to propagate")
	}
}

// --- scanForeignKeys ---

func TestMySQLScanner_ForeignKeys_Composite(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	expectMySQLFullTableScan(t, mock, "posts", mysqlTableScanOpts{
		fkRows: sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME", "UPDATE_RULE", "DELETE_RULE"}).
			AddRow("fk_posts_author", "author_org_id", "memberships", "org_id", "RESTRICT", "CASCADE").
			AddRow("fk_posts_author", "author_user_id", "memberships", "user_id", "RESTRICT", "CASCADE"),
	})

	schema := mustScanWith(t, sc)
	fks := schema.Tables["posts"].ForeignKeys
	if len(fks) != 1 {
		t.Fatalf("expected 1 grouped composite FK, got %d", len(fks))
	}
	fk := fks[0]
	if len(fk.Columns) != 2 || fk.Columns[0] != "author_org_id" || fk.Columns[1] != "author_user_id" {
		t.Errorf("expected ordered composite columns, got %v", fk.Columns)
	}
	if len(fk.RefColumns) != 2 || fk.RefColumns[0] != "org_id" || fk.RefColumns[1] != "user_id" {
		t.Errorf("expected ordered composite ref columns, got %v", fk.RefColumns)
	}
	if fk.OnUpdate != "RESTRICT" || fk.OnDelete != "CASCADE" {
		t.Errorf("unexpected actions: %+v", fk)
	}
}

func TestMySQLScanner_ForeignKeys_NoneAndQueryError(t *testing.T) {
	t.Run("no foreign keys", func(t *testing.T) {
		sc, mock := newMySQLScanner(t)
		expectMySQLFullTableScan(t, mock, "widgets", mysqlTableScanOpts{})
		schema := mustScanWith(t, sc)
		if len(schema.Tables["widgets"].ForeignKeys) != 0 {
			t.Error("expected no foreign keys")
		}
	})

	t.Run("query error", func(t *testing.T) {
		sc, mock := newMySQLScanner(t)
		mock.ExpectQuery("FROM information_schema.TABLES").
			WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("widgets"))
		mock.ExpectQuery("FROM information_schema.STATISTICS").
			WillReturnRows(sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"}))
		mock.ExpectQuery("FROM information_schema.COLUMNS").
			WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "COLUMN_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA", "COLUMN_KEY"}))
		mock.ExpectQuery("FROM information_schema.KEY_COLUMN_USAGE").WillReturnError(errors.New("boom"))

		if _, err := sc.Scan(context.Background()); err == nil {
			t.Error("expected scanForeignKeys query error to propagate")
		}
	})
}

// --- scanChecks: the swallowed-query-error branch ---

func TestMySQLScanner_Checks_Happy(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	expectMySQLFullTableScan(t, mock, "widgets", mysqlTableScanOpts{
		checkRows: sqlmock.NewRows([]string{"CONSTRAINT_NAME", "CHECK_CLAUSE"}).
			AddRow("widgets_price_check", "price > 0"),
	})

	schema := mustScanWith(t, sc)
	checks := schema.Tables["widgets"].Checks
	if len(checks) != 1 || checks[0].Expr != "price > 0" {
		t.Errorf("expected CHECK_CLAUSE preserved verbatim (no CHECK() stripping, unlike Postgres), got %+v", checks)
	}
}

func TestMySQLScanner_Checks_NoneNoError(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	expectMySQLFullTableScan(t, mock, "widgets", mysqlTableScanOpts{})

	schema := mustScanWith(t, sc)
	if len(schema.Tables["widgets"].Checks) != 0 {
		t.Error("expected no checks")
	}
}

func TestMySQLScanner_Checks_QueryErrorIsSwallowed(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.STATISTICS").
		WillReturnRows(sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"}))
	mock.ExpectQuery("FROM information_schema.COLUMNS").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "COLUMN_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA", "COLUMN_KEY"}).
			AddRow("id", "int", "int(11)", nil, nil, nil, "NO", nil, "", "PRI"))
	mock.ExpectQuery("FROM information_schema.KEY_COLUMN_USAGE").
		WillReturnRows(sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME", "UPDATE_RULE", "DELETE_RULE"}))
	// CHECK_CONSTRAINTS doesn't exist on MySQL < 8.0.16 — the query itself
	// errors, and scanChecks must swallow it into (nil, nil), not abort the
	// table scan.
	mock.ExpectQuery("FROM information_schema.CHECK_CONSTRAINTS").
		WillReturnError(errors.New(`Table 'information_schema.CHECK_CONSTRAINTS' doesn't exist`))
	mock.ExpectQuery("FROM information_schema.VIEWS").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "VIEW_DEFINITION"}))
	mock.ExpectQuery("FROM information_schema.TRIGGERS").
		WillReturnRows(sqlmock.NewRows([]string{"TRIGGER_NAME", "EVENT_OBJECT_TABLE", "ACTION_TIMING", "EVENT_MANIPULATION", "ACTION_STATEMENT"}))

	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("expected the swallowed CHECK_CONSTRAINTS error to NOT abort the scan, got %v", err)
	}
	tbl := schema.Tables["widgets"]
	if tbl.Checks != nil {
		t.Errorf("expected nil Checks, got %+v", tbl.Checks)
	}
	// The rest of the table scan must still have completed normally.
	if len(tbl.Columns) != 1 || tbl.Columns[0].Name != "id" {
		t.Errorf("expected the rest of the table scan to complete despite the swallowed checks error, got %+v", tbl)
	}
}

func TestMySQLScanner_Checks_RowScanErrorIsNotSwallowed(t *testing.T) {
	// Scan() fails and returns before ever reaching scanViews/scanTriggers,
	// so (unlike expectMySQLFullTableScan) no trailing globals expectations
	// are registered here.
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("widgets"))
	mock.ExpectQuery("FROM information_schema.STATISTICS").
		WillReturnRows(sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"}))
	mock.ExpectQuery("FROM information_schema.COLUMNS").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "COLUMN_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA", "COLUMN_KEY"}))
	mock.ExpectQuery("FROM information_schema.KEY_COLUMN_USAGE").
		WillReturnRows(sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME", "UPDATE_RULE", "DELETE_RULE"}))
	mock.ExpectQuery("FROM information_schema.CHECK_CONSTRAINTS").
		WillReturnRows(sqlmock.NewRows([]string{"CONSTRAINT_NAME", "CHECK_CLAUSE"}).
			AddRow("c1", "price > 0").
			RowError(0, errors.New("mid-scan boom")))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected a row-scan error on CHECK_CONSTRAINTS (distinct from the query-level error) to propagate, not be swallowed")
	}
}

// --- scanViews ---

func TestMySQLScanner_Views(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}))
	mock.ExpectQuery("FROM information_schema.VIEWS").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "VIEW_DEFINITION"}).
			AddRow("active_users", "select * from users where active").
			AddRow("legacy_view", nil))
	mock.ExpectQuery("FROM information_schema.TRIGGERS").
		WillReturnRows(sqlmock.NewRows([]string{"TRIGGER_NAME", "EVENT_OBJECT_TABLE", "ACTION_TIMING", "EVENT_MANIPULATION", "ACTION_STATEMENT"}))

	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if schema.Views["active_users"].Definition == "" {
		t.Error("expected view definition to be populated")
	}
	if schema.Views["legacy_view"].Definition != "" {
		t.Errorf("expected NULL VIEW_DEFINITION to become empty string, got %q", schema.Views["legacy_view"].Definition)
	}
}

func TestMySQLScanner_Views_QueryError(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}))
	mock.ExpectQuery("FROM information_schema.VIEWS").WillReturnError(errors.New("boom"))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected scanViews error to propagate")
	}
}

// --- scanTriggers: last-row-wins (no multi-event merge, unlike Postgres) ---

func TestMySQLScanner_Triggers_LastRowWinsNoMerge(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}))
	mock.ExpectQuery("FROM information_schema.VIEWS").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "VIEW_DEFINITION"}))
	mock.ExpectQuery("FROM information_schema.TRIGGERS").
		WillReturnRows(sqlmock.NewRows([]string{"TRIGGER_NAME", "EVENT_OBJECT_TABLE", "ACTION_TIMING", "EVENT_MANIPULATION", "ACTION_STATEMENT"}).
			AddRow("trg_dup", "widgets", "BEFORE", "INSERT", "stmt1").
			AddRow("trg_dup", "other_table", "AFTER", "UPDATE", "stmt2"))

	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	trig := schema.Triggers["trg_dup"]
	if trig.Table != "other_table" || trig.Timing != "AFTER" || trig.Event != "UPDATE" || trig.Definition != "stmt2" {
		t.Errorf("expected the second row to fully overwrite the first (no merge), got %+v", trig)
	}
}

func TestMySQLScanner_Triggers_QueryError(t *testing.T) {
	sc, mock := newMySQLScanner(t)
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}))
	mock.ExpectQuery("FROM information_schema.VIEWS").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "VIEW_DEFINITION"}))
	mock.ExpectQuery("FROM information_schema.TRIGGERS").WillReturnError(errors.New("boom"))

	if _, err := sc.Scan(context.Background()); err == nil {
		t.Error("expected scanTriggers error to propagate")
	}
}

// --- helpers ---

type mysqlTableScanOpts struct {
	indexRows  *sqlmock.Rows
	columnRows *sqlmock.Rows
	fkRows     *sqlmock.Rows
	checkRows  *sqlmock.Rows
}

// expectFullTableScan wires up sqlmock expectations for one table's full
// scanTable() call sequence: scanIndexes -> scanColumns -> scanForeignKeys
// -> scanChecks (MySQL's actual order — indexes come first here, unlike
// Postgres, since uniqueCols is threaded into scanColumns).
func expectMySQLFullTableScan(t *testing.T, mock sqlmock.Sqlmock, table string, opts mysqlTableScanOpts) {
	t.Helper()
	mock.ExpectQuery("FROM information_schema.TABLES").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow(table))

	idx := opts.indexRows
	if idx == nil {
		idx = sqlmock.NewRows([]string{"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE"})
	}
	mock.ExpectQuery("FROM information_schema.STATISTICS").WillReturnRows(idx)

	cols := opts.columnRows
	if cols == nil {
		cols = sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "COLUMN_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA", "COLUMN_KEY"})
	}
	mock.ExpectQuery("FROM information_schema.COLUMNS").WillReturnRows(cols)

	fk := opts.fkRows
	if fk == nil {
		fk = sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME", "UPDATE_RULE", "DELETE_RULE"})
	}
	mock.ExpectQuery("FROM information_schema.KEY_COLUMN_USAGE").WillReturnRows(fk)

	checks := opts.checkRows
	if checks == nil {
		checks = sqlmock.NewRows([]string{"CONSTRAINT_NAME", "CHECK_CLAUSE"})
	}
	mock.ExpectQuery("FROM information_schema.CHECK_CONSTRAINTS").WillReturnRows(checks)

	mock.ExpectQuery("FROM information_schema.VIEWS").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "VIEW_DEFINITION"}))
	mock.ExpectQuery("FROM information_schema.TRIGGERS").
		WillReturnRows(sqlmock.NewRows([]string{"TRIGGER_NAME", "EVENT_OBJECT_TABLE", "ACTION_TIMING", "EVENT_MANIPULATION", "ACTION_STATEMENT"}))
}
