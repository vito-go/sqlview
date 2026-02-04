package sqlview_test

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/vito-go/sqlview"

	_ "github.com/lib/pq"
)

func Example() {
	// Connect to database
	db, err := sql.Open("postgres", "postgres://user:pass@localhost/dbname?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// Create SQLView instance
	viewer := sqlview.New(db, "/sqlview")

	// Option 1: Mount to existing ServeMux
	mux := http.NewServeMux()
	viewer.Mount(mux)

	// Start server
	log.Println("DBViewer running at http://localhost:8080/sqlview")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func ExampleDBViewer_Handler() {
	db, _ := sql.Open("postgres", "postgres://user:pass@localhost/dbname?sslmode=disable")
	defer func() { _ = db.Close() }()

	// Option 2: Use standalone Handler
	viewer := sqlview.New(db, "/db")
	handler := viewer.Handler()

	log.Println("DBViewer running at http://localhost:8080/db")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func ExampleDBViewer_Mount() {
	db, _ := sql.Open("postgres", "postgres://user:pass@localhost/dbname?sslmode=disable")
	defer func() { _ = db.Close() }()

	// Create your main router
	mux := http.NewServeMux()

	// Add other API routes
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		// Your API logic
	})

	// Add SQLView (mounted at /admin/db path)
	viewer := sqlview.New(db, "/admin/db")
	viewer.Mount(mux)

	log.Println("Server running at http://localhost:8080")
	log.Println("DBViewer at http://localhost:8080/admin/db")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
