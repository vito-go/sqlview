# SQLView

<div align="center">

🗄️ **A lightweight, embeddable database viewer for Go applications**

[![Go Report Card](https://goreportcard.com/badge/github.com/vito-go/sqlview)](https://goreportcard.com/report/github.com/vito-go/sqlview)
[![GoDoc](https://godoc.org/github.com/vito-go/sqlview?status.svg)](https://godoc.org/github.com/vito-go/sqlview)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

[English](README.md) | [简体中文](README_CN.md)

</div>

---

## ✨ Features

- 🚀 **Zero Dependencies** - Single HTML file embedded, no external assets
- 🔌 **Easy Integration** - Mount to any HTTP server in 2 lines of code
- 🗄️ **Multi-Database Support** - PostgreSQL, MySQL, SQLite
- 🎨 **Modern UI** - Clean, responsive web interface
- 🔍 **Smart Ordering** - Automatically sorts by ID, updated_at, etc.
- 📊 **Table Inspector** - View schema, indexes, DDL, statistics
- 💾 **CSV Export** - Export query results (client-side)
- 🔒 **Read-Only** - Only SELECT queries allowed for safety
- 🎯 **Context Menu** - Right-click for quick actions
- 🌐 **Multi-Database Mode** - Switch between databases on the fly

## 📸 Screenshots

### Main Interface
Browse databases, tables, and run queries with a modern web UI.

### Context Menu
Right-click any table for quick actions: view data, schema, indexes, DDL, or statistics.

### Table Statistics
View row counts, table sizes, and index information.

## 🚀 Quick Start

### Installation

```bash
go get github.com/vito-go/sqlview
```

### Basic Usage

```go
package main

import (
    "database/sql"
    "log"
    "net/http"

    "github.com/vito-go/sqlview"
    _ "github.com/lib/pq" // PostgreSQL driver
)

func main() {
    // Connect to your database
    db, _ := sql.Open("postgres",
        "postgres://user:pass@localhost:5432/mydb?sslmode=disable")

    // Create SQLView instance (auto-detects database type)
    viewer := sqlview.New(db, "/sqlview")

    // Mount to your HTTP server
    mux := http.NewServeMux()
    viewer.Mount(mux)

    log.Println("SQLView running at http://localhost:8080/sqlview")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

That's it! Open http://localhost:8080/sqlview in your browser.

## 📚 Documentation

### Creating a Viewer

#### Option 1: Auto-detect Database Type (Recommended)

```go
viewer := sqlview.New(db, "/sqlview")
```

SQLView automatically detects PostgreSQL, MySQL, or SQLite.

#### Option 2: Multi-Database Mode

Enable browsing multiple databases by providing a DSN without database name:

```go
// PostgreSQL: omit database name
dsn := "postgres://user:pass@localhost:5432/?sslmode=disable"
viewer := sqlview.NewWithDSN(db, "/sqlview", dsn, "postgres")

// MySQL: omit database name
dsn := "user:pass@tcp(host:3306)/?parseTime=true"
viewer := sqlview.NewWithDSN(db, "/sqlview", dsn, "mysql")
```

### Mounting to HTTP Server

```go
// Option 1: Mount to existing ServeMux
mux := http.NewServeMux()
viewer.Mount(mux)

// Option 2: Get standalone handler
handler := viewer.Handler()
http.ListenAndServe(":8080", handler)
```

### Adding Middleware (Authentication, Logging, etc.)

```go
func authMiddleware(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
    token := r.Header.Get("Authorization")
    if token == "" {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return r, false // Stop processing
    }
    return r, true // Continue
}

// Mount with middleware
viewer.Mount(mux, authMiddleware)
```

## 🗄️ Database Support

| Database   | Single-DB | Multi-DB | Features |
|------------|-----------|----------|----------|
| PostgreSQL | ✅        | ✅       | Full support with pg_* system tables |
| MySQL      | ✅        | ✅       | Full support with SHOW commands |
| SQLite     | ✅        | ❌       | Single file per connection |

### Driver Requirements

SQLView doesn't import database drivers to avoid forcing dependencies. Import the drivers you need:

```go
import (
    _ "github.com/lib/pq"                // PostgreSQL
    _ "github.com/go-sql-driver/mysql"   // MySQL
    _ "modernc.org/sqlite"               // SQLite (pure Go, no CGO)
    // or
    // _ "github.com/mattn/go-sqlite3"   // SQLite (CGO, faster)
)
```

## 🎯 Features

### Table Operations

**Right-click any table** to access:

- 📊 **View Data** - Display table contents (first 100 rows by default)
- 📐 **View Schema** - Column names, types, nullable, defaults
- 🔑 **View Indexes** - Index names, types, columns, uniqueness
- 📝 **View DDL** - Complete CREATE TABLE and CREATE INDEX statements
- 📈 **View Statistics** - Row count, table size, index size
- 📋 **Copy Table Name** - Copy to clipboard

### SQL Query Editor

- ✏️ Multi-line SQL editor with syntax highlighting
- ⌨️ Keyboard shortcuts: `Ctrl/Cmd + Enter` to execute
- 🔒 Read-only mode: Only SELECT and WITH queries allowed
- 💾 CSV export for query results
- 📊 Formatted result table

### Smart Features

- 🔍 **Table Search** - Real-time table name filtering
- 🎯 **Smart Ordering** - Auto-sorts by id, updated_at, created_at DESC
- 🌐 **Multi-Database Switching** - Browse different databases
- 📱 **Responsive Design** - Works on desktop and mobile
- ⌨️ **Keyboard Shortcuts** - ESC to close modals/menus

## 🔒 Security

⚠️ **Important Security Notes:**

1. SQLView only allows `SELECT` and `WITH` queries
2. No INSERT, UPDATE, DELETE, or DDL operations
3. **Recommended**: Use authentication middleware in production
4. **Recommended**: Restrict access to internal networks only

```go
// Example: Basic auth middleware
func basicAuth(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
    user, pass, ok := r.BasicAuth()
    if !ok || user != "admin" || pass != "secret" {
        w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return r, false
    }
    return r, true
}

viewer.Mount(mux, basicAuth)
```

## 📖 API Reference

### Types

```go
// New creates a SQLView instance with auto-detected database type
func New(db *sql.DB, basePath string) *SQLView

// NewWithDSN creates a SQLView instance with explicit configuration
func NewWithDSN(db *sql.DB, basePath, dsn, driverName string) *SQLView

// Mount registers routes to an HTTP mux with optional middleware
func (sv *SQLView) Mount(mux interface{
    HandleFunc(string, func(http.ResponseWriter, *http.Request))
}, preHandles ...PreHandle)

// Handler returns a standalone HTTP handler
func (sv *SQLView) Handler() http.Handler

// PreHandle is a middleware function type
type PreHandle func(w http.ResponseWriter, r *http.Request) (*http.Request, bool)
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/connection-info` | GET | Database connection info |
| `/api/databases` | GET | List databases/schemas |
| `/api/tables?database=<db>` | GET | List tables |
| `/api/table-data?database=<db>&table=<name>` | GET | Table data (100 rows) |
| `/api/table-schema?database=<db>&table=<name>` | GET | Table schema |
| `/api/table-indexes?database=<db>&table=<name>` | GET | Table indexes |
| `/api/table-ddl?database=<db>&table=<name>` | GET | CREATE TABLE DDL |
| `/api/table-stats?database=<db>&table=<name>` | GET | Table statistics |
| `/api/query` | POST | Execute SELECT query |

## 🛠️ Examples

See [examples](examples/) directory for complete examples:

- [PostgreSQL](examples/postgres/) - Basic PostgreSQL example
- [MySQL](examples/mysql/) - Basic MySQL example
- [SQLite](examples/sqlite/) - In-memory SQLite example
- [Multi-Database](examples/multi-db/) - Multi-database browsing
- [With Middleware](examples/middleware/) - Authentication example

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by the need for a simple, embeddable database viewer
- Built with vanilla JavaScript (no frameworks!)
- Uses Go's `embed` package for zero-dependency deployment

## 📮 Contact

- GitHub: [@vito-go](https://github.com/vito-go)
- Project: [github.com/vito-go/sqlview](https://github.com/vito-go/sqlview)

---

<div align="center">
Made with ❤️ by the Go community
</div>
