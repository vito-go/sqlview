package sqlview_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vito-go/sqlview"

	_ "modernc.org/sqlite"
)

func TestSingleDatabaseMode(t *testing.T) {
	// Create in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Test single database mode using New()
	t.Run("SingleDatabaseMode_ShouldReturnOnlyCurrentDatabase", func(t *testing.T) {
		// Create DBViewer using New (single database mode)
		viewer := sqlview.New(db, "/dbviewer")

		// Create a test HTTP request to /api/databases
		req := httptest.NewRequest(http.MethodGet, "/dbviewer/api/databases", nil)
		w := httptest.NewRecorder()

		// Create handler and serve the request
		mux := http.NewServeMux()
		viewer.Mount(mux)
		mux.ServeHTTP(w, req)

		// Check response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Parse response
		var databases []string
		if err := json.Unmarshal(w.Body.Bytes(), &databases); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// In single database mode, should only return current database
		if len(databases) != 1 {
			t.Errorf("Expected 1 database in single mode, got %d: %v", len(databases), databases)
		}

		if databases[0] != "main" {
			t.Errorf("Expected database name 'main', got '%s'", databases[0])
		}

		t.Logf("Single database mode correctly returned: %v", databases)
	})

	// Test multi-database mode using NewWithDSN()
	t.Run("MultiDatabaseMode_ShouldReturnAllDatabases", func(t *testing.T) {
		// For SQLite, multi-database mode doesn't really apply,
		// but we can test that NewWithDSN with a database name still works
		viewer := sqlview.NewWithDSN(db, "/dbviewer", "/tmp/test.db", "sqlite")

		// Create a test HTTP request to /api/databases
		req := httptest.NewRequest(http.MethodGet, "/dbviewer/api/databases", nil)
		w := httptest.NewRecorder()

		// Create handler and serve the request
		mux := http.NewServeMux()
		viewer.Mount(mux)
		mux.ServeHTTP(w, req)

		// Check response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		// Parse response
		var databases []string
		if err := json.Unmarshal(w.Body.Bytes(), &databases); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// SQLite should still return only "main"
		if len(databases) != 1 {
			t.Errorf("Expected 1 database, got %d: %v", len(databases), databases)
		}

		t.Logf("Multi-database mode (SQLite) returned: %v", databases)
	})
}
