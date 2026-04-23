package sqlview_test

import (
	"database/sql"
	"testing"

	"github.com/vito-go/sqlview"

	_ "modernc.org/sqlite"
)

func TestSQLite_PRAGMACommands(t *testing.T) {
	// Create in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer db.Close()

	// Create a test table with indexes
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			age INTEGER
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Create an index
	_, err = db.Exec(`CREATE INDEX idx_users_name ON users(name)`)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Insert some test data
	_, err = db.Exec(`
		INSERT INTO users (name, email, age) VALUES
		('Alice', 'alice@example.com', 25),
		('Bob', 'bob@example.com', 30)
	`)
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	// Get SQLite adapter for testing
	adapter := sqlview.NewTestAdapter("sqlite")

	// Test GetDatabases
	t.Run("GetDatabases", func(t *testing.T) {
		databases, err := adapter.GetDatabases(db)
		if err != nil {
			t.Errorf("GetDatabases failed: %v", err)
		}
		if len(databases) == 0 {
			t.Error("Expected at least one database")
		}
		t.Logf("Databases: %v", databases)
	})

	// Test GetTables
	t.Run("GetTables", func(t *testing.T) {
		tables, err := adapter.GetTables(db, "main")
		if err != nil {
			t.Errorf("GetTables failed: %v", err)
		}
		if len(tables) == 0 {
			t.Error("Expected at least one table")
		}
		t.Logf("Tables: %v", tables)
	})

	// Test GetTableColumns (uses PRAGMA table_info)
	t.Run("GetTableColumns", func(t *testing.T) {
		columns, err := adapter.GetTableColumns(db, "main", "users")
		if err != nil {
			t.Errorf("GetTableColumns failed: %v", err)
		}
		if len(columns) != 4 {
			t.Errorf("Expected 4 columns, got %d", len(columns))
		}
		t.Logf("Columns: %v", columns)
	})

	// Test GetTableSchema (uses PRAGMA table_info)
	t.Run("GetTableSchema", func(t *testing.T) {
		schema, err := adapter.GetTableSchema(db, "main", "users")
		if err != nil {
			t.Errorf("GetTableSchema failed: %v", err)
		}
		if len(schema) != 4 {
			t.Errorf("Expected 4 columns in schema, got %d", len(schema))
		}
		for _, col := range schema {
			t.Logf("Column: %s, Type: %s, Nullable: %v, Default: %s",
				col.Name, col.Type, col.Nullable, col.Default)
		}
	})

	// Test GetTableIndexes (uses PRAGMA index_list and index_info)
	// This is the main test for the bug fix
	t.Run("GetTableIndexes", func(t *testing.T) {
		indexes, err := adapter.GetTableIndexes(db, "main", "users")
		if err != nil {
			t.Errorf("GetTableIndexes failed: %v", err)
		}
		if len(indexes) == 0 {
			t.Error("Expected at least one index")
		}
		for _, idx := range indexes {
			t.Logf("Index: %s, Type: %s, Unique: %v, Columns: %v",
				idx.Name, idx.Type, idx.Unique, idx.Columns)
			// Verify that index has columns
			if len(idx.Columns) == 0 {
				t.Errorf("Index %s should have at least one column", idx.Name)
			}
		}
	})

	// Test with special characters in table name
	t.Run("SpecialCharactersInTableName", func(t *testing.T) {
		tableName := "test_table"
		_, err = db.Exec(`CREATE TABLE ` + tableName + ` (id INTEGER PRIMARY KEY, value TEXT)`)
		if err != nil {
			t.Fatalf("Failed to create test table: %v", err)
		}

		_, err = db.Exec(`CREATE INDEX idx_test ON ` + tableName + `(value)`)
		if err != nil {
			t.Fatalf("Failed to create index: %v", err)
		}

		// Test all PRAGMA commands with this table
		columns, err := adapter.GetTableColumns(db, "main", tableName)
		if err != nil {
			t.Errorf("GetTableColumns failed for special table: %v", err)
		}
		if len(columns) != 2 {
			t.Errorf("Expected 2 columns, got %d", len(columns))
		}

		indexes, err := adapter.GetTableIndexes(db, "main", tableName)
		if err != nil {
			t.Errorf("GetTableIndexes failed for special table: %v", err)
		}
		if len(indexes) == 0 {
			t.Error("Expected at least one index")
		}

		t.Logf("Successfully handled table: %s", tableName)
	})

	// Test GetAutoIncrementColumns: INTEGER PRIMARY KEY is a rowid alias
	t.Run("GetAutoIncrementColumns", func(t *testing.T) {
		cols, err := adapter.GetAutoIncrementColumns(db, "main", "users")
		if err != nil {
			t.Fatalf("GetAutoIncrementColumns failed: %v", err)
		}
		if len(cols) != 1 || cols[0] != "id" {
			t.Errorf("Expected [id], got %v", cols)
		}
	})

	// Verify ordering prefers a non-id auto-increment column over a plain id
	t.Run("BuildTableDataQueryWithOrder_AutoIncBeforeId", func(t *testing.T) {
		if _, err := db.Exec(`
			CREATE TABLE events (
				id TEXT,
				seq INTEGER PRIMARY KEY,
				payload TEXT
			)
		`); err != nil {
			t.Fatalf("Failed to create events table: %v", err)
		}

		columns, err := adapter.GetTableColumns(db, "main", "events")
		if err != nil {
			t.Fatalf("GetTableColumns failed: %v", err)
		}
		autoInc, err := adapter.GetAutoIncrementColumns(db, "main", "events")
		if err != nil {
			t.Fatalf("GetAutoIncrementColumns failed: %v", err)
		}

		query := adapter.BuildTableDataQueryWithOrder("main", "events", columns, autoInc, 10)
		// seq (auto-increment) should be chosen, not id (plain text column)
		if want := "ORDER BY seq DESC"; !contains(query, want) {
			t.Errorf("Expected query to contain %q, got: %s", want, query)
		}
	})
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
