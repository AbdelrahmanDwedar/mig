package scanner_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/AbdelrahmanDwedar/mig/internal/scanner"
	_ "github.com/mattn/go-sqlite3"
)

func openSQLite(t *testing.T, name string, ddl ...string) *sql.DB {
	t.Helper()
	t.Cleanup(func() { os.Remove(name) })

	db, err := sql.Open("sqlite3", name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	for _, stmt := range ddl {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed DDL failed: %v\nstatement: %s", err, stmt)
		}
	}
	return db
}

func mustScan(t *testing.T, db *sql.DB) *scanner.Schema {
	t.Helper()
	sc, err := scanner.New("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func TestSQLiteScanner_EmptyDatabase(t *testing.T) {
	db := openSQLite(t, "test_scanner_empty.db")
	schema := mustScan(t, db)

	if len(schema.Tables) != 0 {
		t.Errorf("expected no tables, got %d", len(schema.Tables))
	}
	if len(schema.Views) != 0 || len(schema.Triggers) != 0 || len(schema.Sequences) != 0 {
		t.Error("expected empty views/triggers/sequences maps for a fresh database")
	}
}

func TestSQLiteScanner_ExcludesMigrationsTable(t *testing.T) {
	db := openSQLite(t, "test_scanner_excludes.db",
		`CREATE TABLE _migrations (uuid CHAR(36) PRIMARY KEY, migration TEXT, batch INTEGER)`,
		`CREATE TABLE widgets (id INTEGER PRIMARY KEY)`,
	)
	schema := mustScan(t, db)

	if _, ok := schema.Tables["_migrations"]; ok {
		t.Error("expected _migrations to be excluded from scanned tables")
	}
	if _, ok := schema.Tables["widgets"]; !ok {
		t.Error("expected widgets table to be present")
	}
}

func TestSQLiteScanner_ColumnsAndTypes(t *testing.T) {
	db := openSQLite(t, "test_scanner_columns.db", `
		CREATE TABLE widgets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			price NUMERIC(10,2) DEFAULT 0,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			ident TEXT,
			payload BLOB
		)`)
	schema := mustScan(t, db)

	tbl, ok := schema.Tables["widgets"]
	if !ok {
		t.Fatal("expected widgets table")
	}

	cols := map[string]scanner.Column{}
	for _, c := range tbl.Columns {
		cols[c.Name] = c
	}

	id := cols["id"]
	if !id.PrimaryKey || !id.AutoIncrement || id.Type != "integer" {
		t.Errorf("id column: got PrimaryKey=%v AutoIncrement=%v Type=%q, want PK+AutoIncrement+integer", id.PrimaryKey, id.AutoIncrement, id.Type)
	}

	name := cols["name"]
	if name.Type != "string" || name.Length != 100 {
		t.Errorf("name column: got Type=%q Length=%d, want string(100)", name.Type, name.Length)
	}
	if name.Nullable == nil || *name.Nullable {
		t.Error("name column: expected NOT NULL (nullable=false)")
	}

	desc := cols["description"]
	if desc.Type != "text" {
		t.Errorf("description column: got Type=%q, want text", desc.Type)
	}
	if desc.Nullable == nil || !*desc.Nullable {
		t.Error("description column: expected nullable=true")
	}

	price := cols["price"]
	if price.Type != "decimal" || price.Precision != 10 || price.Scale != 2 {
		t.Errorf("price column: got Type=%q Precision=%d Scale=%d, want decimal(10,2)", price.Type, price.Precision, price.Scale)
	}
	if price.Default == nil || price.Default.Expr != "0" {
		t.Errorf("price column: expected Default.Expr=\"0\", got %+v", price.Default)
	}

	active := cols["is_active"]
	if active.Type != "boolean" {
		t.Errorf("is_active column: got Type=%q, want boolean", active.Type)
	}

	created := cols["created_at"]
	if created.Type != "timestamp" {
		t.Errorf("created_at column: got Type=%q, want timestamp", created.Type)
	}
	if created.Default == nil || created.Default.Expr != "CURRENT_TIMESTAMP" {
		t.Errorf("created_at column: expected Default.Expr=CURRENT_TIMESTAMP, got %+v", created.Default)
	}

	// A second TEXT-affinity column with no abstract-type ambiguity here,
	// but exercises the raw-native fallback contract: BLOB has no
	// abstract equivalent at all in typeMap.
	payload := cols["payload"]
	if payload.Type != scanner.RawType || payload.NativeType != "BLOB" {
		t.Errorf("payload column: got Type=%q NativeType=%q, want raw/BLOB", payload.Type, payload.NativeType)
	}
}

func TestSQLiteScanner_CompositePrimaryKey(t *testing.T) {
	db := openSQLite(t, "test_scanner_composite_pk.db", `
		CREATE TABLE memberships (
			org_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT,
			PRIMARY KEY (org_id, user_id)
		)`)
	schema := mustScan(t, db)

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

func TestSQLiteScanner_UniqueColumn(t *testing.T) {
	db := openSQLite(t, "test_scanner_unique.db", `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email VARCHAR(255) NOT NULL UNIQUE
		)`)
	schema := mustScan(t, db)

	tbl := schema.Tables["users"]
	var email *scanner.Column
	for i := range tbl.Columns {
		if tbl.Columns[i].Name == "email" {
			email = &tbl.Columns[i]
		}
	}
	if email == nil || !email.Unique {
		t.Error("expected email column to be reported as Unique")
	}
	// The implicit unique index backing a single-column UNIQUE constraint
	// has no user-chosen name — it should fold into Column.Unique, not
	// appear as a separate Index entry.
	if len(tbl.Indexes) != 0 {
		t.Errorf("expected no displayed indexes for the implicit unique constraint, got %v", tbl.Indexes)
	}
}

func TestSQLiteScanner_CompositeUniqueConstraint(t *testing.T) {
	db := openSQLite(t, "test_scanner_composite_unique.db", `
		CREATE TABLE memberships (
			org_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			UNIQUE (org_id, user_id)
		)`)
	schema := mustScan(t, db)

	tbl := schema.Tables["memberships"]
	if len(tbl.Indexes) != 1 {
		t.Fatalf("expected the composite unique constraint to surface as an Index (can't collapse to a per-column flag), got %d indexes", len(tbl.Indexes))
	}
	if !tbl.Indexes[0].Unique || len(tbl.Indexes[0].Columns) != 2 {
		t.Errorf("expected a unique 2-column index, got %+v", tbl.Indexes[0])
	}
}

func TestSQLiteScanner_ExplicitIndex(t *testing.T) {
	db := openSQLite(t, "test_scanner_explicit_index.db", `
		CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT);
		CREATE INDEX idx_posts_title ON posts(title);
	`)
	schema := mustScan(t, db)

	tbl := schema.Tables["posts"]
	if len(tbl.Indexes) != 1 {
		t.Fatalf("expected 1 explicit index, got %d", len(tbl.Indexes))
	}
	idx := tbl.Indexes[0]
	if idx.Name != "idx_posts_title" || idx.Unique || len(idx.Columns) != 1 || idx.Columns[0] != "title" {
		t.Errorf("unexpected index: %+v", idx)
	}
}

func TestSQLiteScanner_ForeignKey(t *testing.T) {
	db := openSQLite(t, "test_scanner_fk.db", `
		CREATE TABLE users (id INTEGER PRIMARY KEY);
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE RESTRICT
		);
	`)
	schema := mustScan(t, db)

	tbl := schema.Tables["posts"]
	if len(tbl.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key, got %d", len(tbl.ForeignKeys))
	}
	fk := tbl.ForeignKeys[0]
	if fk.RefTable != "users" || len(fk.Columns) != 1 || fk.Columns[0] != "user_id" ||
		len(fk.RefColumns) != 1 || fk.RefColumns[0] != "id" {
		t.Fatalf("unexpected foreign key shape: %+v", fk)
	}
	if fk.OnDelete != "CASCADE" || fk.OnUpdate != "RESTRICT" {
		t.Errorf("expected ON DELETE CASCADE / ON UPDATE RESTRICT, got OnDelete=%q OnUpdate=%q", fk.OnDelete, fk.OnUpdate)
	}
}

func TestSQLiteScanner_ViewAndTrigger(t *testing.T) {
	db := openSQLite(t, "test_scanner_view_trigger.db", `
		CREATE TABLE users (id INTEGER PRIMARY KEY, active BOOLEAN);
		CREATE VIEW active_users AS SELECT * FROM users WHERE active = 1;
		CREATE TRIGGER trg_users_touch AFTER UPDATE ON users BEGIN SELECT 1; END;
	`)
	schema := mustScan(t, db)

	view, ok := schema.Views["active_users"]
	if !ok {
		t.Fatal("expected active_users view")
	}
	if view.Definition == "" {
		t.Error("expected view definition to be populated")
	}

	trig, ok := schema.Triggers["trg_users_touch"]
	if !ok {
		t.Fatal("expected trg_users_touch trigger")
	}
	if trig.Table != "users" || trig.Timing != "AFTER" || trig.Event != "UPDATE" {
		t.Errorf("unexpected trigger metadata: %+v", trig)
	}
}

func TestSQLiteScanner_NoSequences(t *testing.T) {
	db := openSQLite(t, "test_scanner_no_sequences.db", `CREATE TABLE widgets (id INTEGER PRIMARY KEY)`)
	schema := mustScan(t, db)

	if len(schema.Sequences) != 0 {
		t.Errorf("expected no sequences on SQLite, got %d", len(schema.Sequences))
	}
}
