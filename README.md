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

### Core Features
- 🚀 **Zero Dependencies** - Single HTML file embedded, no external assets
- 🔌 **Easy Integration** - Mount to any HTTP server in 2 lines of code
- 🗄️ **Multi-Database Support** - PostgreSQL, MySQL, SQLite
- 🔒 **Read-Only** - Only SELECT queries allowed for safety
- 🌐 **Multi-Database Mode** - Switch between databases on the fly

### UI Features (v0.1.0+)
- 🎨 **Modern Tabbed Interface** - SQL Query, Schema Explorer, Charts
- 📋 **Schema Explorer** - Comprehensive database structure viewer with stats
- 📊 **Data Visualization** - Interactive charts (Bar, Line, Pie, Doughnut)
- ✨ **SQL Syntax Highlighting** - Powered by CodeMirror with multiple themes
- 📜 **Query History** - Local storage of recent 100 queries
- 🌓 **Dark/Light Theme** - Toggle between themes with persistent preference
- 💾 **Multi-Format Export** - Export to CSV or JSON (client-side)
- 🎯 **Smart Context Menu** - Quick access to common operations
- 🔍 **Collapsible Sidebar** - More space when you need it
- 📱 **Responsive Design** - Works beautifully on all screen sizes

## 📸 Screenshots

### SQL Query Tab
Execute SQL queries with syntax highlighting, view results in formatted tables, and export to CSV/JSON.

### Schema Explorer Tab
Explore database structure with detailed column information, indexes, DDL statements, and table statistics.

### Charts Tab
Visualize your data with interactive charts - choose from bar, line, pie, or doughnut charts with configurable axes and aggregations.

### Dark Theme
Beautiful dark mode support across all tabs and components.

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

### Tab 1: SQL Query

**Advanced SQL Editor**
- ✨ **Syntax Highlighting** - CodeMirror integration with multiple themes
- ⌨️ **Keyboard Shortcuts** - `Ctrl/Cmd + Enter` to execute
- 📜 **Query History** - Access your recent 100 queries
  - Single-click to load query
  - Double-click to load and execute
  - Delete individual or all history
- 🔒 **Read-Only Mode** - Only SELECT and WITH queries allowed
- 💾 **Export Results** - CSV and JSON formats
- 🎨 **Comment Support** - Write comments with `--`, auto-filtered before execution
- 🖱️ **Interactive Results Table** - Click any cell to view full content in popup with copy support

### Tab 2: Schema Explorer

**Comprehensive Structure Viewer**
- 📊 **Overview Section** - Row count, table size, index size statistics
- 📝 **Columns Section** - Detailed column information with badges (PK, UNIQUE, INDEX)
- 🔑 **Indexes Section** - Complete index details with types and columns
- 📋 **DDL Section** - CREATE TABLE statements with one-click copy
- 🔍 **Table Search** - Real-time filtering of table list
- 🎯 **Quick Actions** - Copy name, view data buttons

### Tab 3: Charts

**Interactive Data Visualization**
- 📊 **Chart Types** - Bar, Line, Pie, Doughnut charts
- 🎨 **Configurable Axes** - Select any column for X/Y axes
- 📈 **Aggregations** - COUNT, SUM, AVG, MAX, MIN
- 📅 **Time Aggregation** - Group date/time columns by Day, Week, Month, or Year (supports PostgreSQL, MySQL, SQLite)
- 🎯 **Smart Recommendations** - Auto-detect numeric columns for Y-axis
- 📉 **Data Limits** - Choose 10, 20, 50, or 100 records
- 🌓 **Theme Aware** - Charts adapt to light/dark theme

### Table Operations

**Right-click any table** for quick access:
- 📊 **View Data** - Display table contents (auto-switches to Query tab)
- 🔍 **Open in Schema Explorer** - Jump to full structure view
- 📋 **Copy Table Name** - Copy to clipboard

### Smart Features

- 🔍 **Table Search** - Real-time table name filtering
- 🎯 **Smart Ordering** - Auto-sorts by id, updated_at, created_at DESC
- 🌐 **Multi-Database Switching** - Browse different databases
- 🌓 **Theme Toggle** - Switch between dark/light themes (persistent)
- 📱 **Responsive Design** - Works beautifully on all screen sizes
- ⌨️ **Keyboard Shortcuts** - `Ctrl/Cmd + Enter` to execute, `ESC` to close
- 🔄 **Collapsible Sidebar** - Maximize content area when needed

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
