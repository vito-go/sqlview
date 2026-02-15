package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/vito-go/sqlview"
)

// Example demonstrating binary (BYTEA) column encoding feature
func main() {
	// Connect to PostgreSQL
	dsn := "postgres://user:password@localhost:5432/testdb?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create a test table with BYTEA columns
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_binary (
			id SERIAL PRIMARY KEY,
			encryption_key BYTEA,  -- Contains "key" - will default to hex
			user_avatar BYTEA,      -- No "key" - will default to base64
			api_key BYTEA,          -- Contains "key" - will default to hex
			file_data BYTEA         -- No "key" - will default to base64
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Insert some test data
	_, err = db.Exec(`
		INSERT INTO test_binary (encryption_key, user_avatar, api_key, file_data) VALUES
		($1, $2, $3, $4)
	`,
		[]byte{0xDE, 0xAD, 0xBE, 0xEF}, // encryption_key (will show as hex)
		[]byte("Hello, World!"),          // user_avatar (will show as base64)
		[]byte{0xCA, 0xFE, 0xBA, 0xBE}, // api_key (will show as hex)
		[]byte{0x89, 0x50, 0x4E, 0x47}, // file_data (PNG header, will show as base64)
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create DBViewer
	viewer := sqlview.New(db, "/dbviewer")

	// Mount to HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dbviewer", http.StatusFound)
	})
	viewer.Mount(http.DefaultServeMux)

	fmt.Println("Server started at http://localhost:8080")
	fmt.Println("Navigate to http://localhost:8080/dbviewer")
	fmt.Println("\nFeatures demonstrated:")
	fmt.Println("1. Binary columns with 'key' in name (encryption_key, api_key) default to HEX encoding")
	fmt.Println("2. Other binary columns (user_avatar, file_data) default to Base64 encoding")
	fmt.Println("3. Users can switch encoding (Raw/Hex/Base64) via dropdown in column header")
	fmt.Println("4. Encoding preference is saved in browser localStorage per database/table/column")
	log.Fatal(http.ListenAndServe(":8080", nil))
}