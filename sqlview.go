package sqlview

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

//go:embed index.html
var frontendFS embed.FS

// DBViewer provides an embedded web UI for querying database
type DBViewer struct {
	db        *sql.DB
	basePath  string
	dsnPrefix string    // DSN prefix for multi-database mode (e.g., "postgres://user:pass@host:5432/")
	adapter   dbAdapter // Database-specific adapter
}

type PreHandle func(w http.ResponseWriter, r *http.Request) (newReq *http.Request, next bool)

// New creates a new DBViewer instance (single database mode)
// Automatically detects the database type by querying the database
func New(db *sql.DB, basePath string) *DBViewer {
	if basePath == "" {
		basePath = "/dbviewer"
	}
	// Ensure basePath starts with / and doesn't end with /
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimSuffix(basePath, "/")

	// Auto-detect database type
	dbType := detectDatabaseType(db)

	return &DBViewer{
		db:       db,
		basePath: basePath,
		adapter:  getAdapter(dbType),
	}
}

// NewWithDSN creates a DBViewer with DSN for automatic database detection
// If DSN doesn't specify a database name (e.g., "postgres://user:pass@host:5432/?params"),
// multi-database mode is automatically enabled.
//
// Supported DSN formats:
//   - PostgreSQL: "postgres://user:pass@host:5432/?sslmode=disable" (multi-db)
//     "postgres://user:pass@host:5432/mydb?sslmode=disable" (single-db)
//   - MySQL:      "user:pass@tcp(host:3306)/?parseTime=true" (multi-db)
//     "user:pass@tcp(host:3306)/mydb?parseTime=true" (single-db)
//   - SQLite:     "/path/to/database.db" (single-db only)
//
// driverName: "postgres", "mysql", "sqlite3", etc. (leave empty for auto-detection)
func NewWithDSN(db *sql.DB, basePath, dsn, driverName string) *DBViewer {
	viewer := New(db, basePath)

	// Detect database type and parse DSN
	dbType, dsnPrefix := parseDSN(dsn, driverName)
	viewer.adapter = getAdapter(dbType)
	viewer.dsnPrefix = dsnPrefix
	return viewer
}

// parseDSN extracts database type and DSN prefix for multi-database support
// Returns: (dbType, dsnPrefix)
// If dsnPrefix is empty, multi-database mode is disabled
func parseDSN(dsn, driverName string) (string, string) {
	if driverName == "" {
		driverName = detectDriverFromDSN(dsn)
	}

	switch driverName {
	case "postgres", "postgresql", "pgx":
		// Parse PostgreSQL URL: postgres://user:pass@host:5432/[dbname]?params
		u, err := url.Parse(dsn)
		if err != nil {
			// Fallback to string parsing if URL parsing fails
			return "postgres", ""
		}

		// Check if database name is empty (path is "/" or "/?params" or just "?params")
		dbName := strings.TrimPrefix(u.Path, "/")
		if dbName == "" {
			// Multi-database mode: return full DSN
			return "postgres", dsn
		}
		// Single-database mode
		return "postgres", ""

	case "mysql":
		// MySQL DSN: user:pass@tcp(host:3306)/[dbname]?params
		// Not a standard URL, parse by finding the last '/'
		if idx := strings.LastIndex(dsn, "/"); idx != -1 {
			afterSlash := dsn[idx+1:]
			// If empty or starts with '?', this is multi-database mode
			if afterSlash == "" || strings.HasPrefix(afterSlash, "?") {
				return "mysql", dsn
			}
		}
		return "mysql", ""

	case "sqlite3", "sqlite":
		// SQLite doesn't support multi-database mode
		return "sqlite", ""

	default:
		return "postgres", ""
	}
}

// detectDriverFromDSN tries to detect the database driver from DSN format
func detectDriverFromDSN(dsn string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return "postgres"
	}
	if strings.Contains(dsn, "@tcp(") {
		return "mysql"
	}
	if strings.HasSuffix(dsn, ".db") || strings.HasSuffix(dsn, ".sqlite") || strings.HasSuffix(dsn, ".sqlite3") {
		return "sqlite"
	}
	return "postgres" // default
}

// detectDatabaseType detects the database type by querying the database
func detectDatabaseType(db *sql.DB) string {
	// Try SQLite first (most specific)
	var sqliteVersion string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&sqliteVersion); err == nil {
		return "sqlite"
	}

	// Try PostgreSQL
	var pgVersion string
	if err := db.QueryRow("SELECT version()").Scan(&pgVersion); err == nil {
		if strings.Contains(strings.ToLower(pgVersion), "postgresql") {
			return "postgres"
		}
		// MySQL also supports SELECT version()
		if strings.Contains(strings.ToLower(pgVersion), "mysql") ||
			strings.Contains(strings.ToLower(pgVersion), "mariadb") {
			return "mysql"
		}
	}

	// Try MySQL specific query
	var mysqlVersion string
	if err := db.QueryRow("SELECT DATABASE()").Scan(&mysqlVersion); err == nil {
		return "mysql"
	}

	// Default to postgres if detection fails
	return "postgres"
}

// Mount registers the DBViewer routes to the provided ServeMux
// preHandles are optional middleware functions that will be applied to all routes
func (dv *DBViewer) Mount(mux interface {
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}, preHandles ...PreHandle) {
	// API routes - apply preHandles to all routes
	handleFunc(mux, http.MethodGet+" "+dv.basePath+"/api/connection-info", dv.handleConnectionInfo, preHandles...)
	handleFunc(mux, http.MethodGet+" "+dv.basePath+"/api/databases", dv.handleDatabases, preHandles...)
	handleFunc(mux, http.MethodGet+" "+dv.basePath+"/api/tables", dv.handleTables, preHandles...)
	handleFunc(mux, http.MethodGet+" "+dv.basePath+"/api/table-data", dv.handleTableData, preHandles...)
	handleFunc(mux, http.MethodPost+" "+dv.basePath+"/api/query", dv.handleQuery, preHandles...)

	// New table info endpoints
	handleFunc(mux, http.MethodGet+" "+dv.basePath+"/api/table-schema", dv.handleTableSchema, preHandles...)
	handleFunc(mux, http.MethodGet+" "+dv.basePath+"/api/table-indexes", dv.handleTableIndexes, preHandles...)
	handleFunc(mux, http.MethodGet+" "+dv.basePath+"/api/table-ddl", dv.handleTableDDL, preHandles...)
	handleFunc(mux, http.MethodGet+" "+dv.basePath+"/api/table-stats", dv.handleTableStats, preHandles...)

	// Static files
	fileServer := http.FileServer(http.FS(frontendFS))
	handleFunc(mux, dv.basePath+"/", http.StripPrefix(dv.basePath, fileServer).ServeHTTP, preHandles...)
}
func handleFunc(h interface {
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}, pattern string, handler func(http.ResponseWriter, *http.Request), pres ...PreHandle) {
	h.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		nextR := r
		for _, pre := range pres {
			var next bool
			nextR, next = pre(w, r)
			if !next {
				return
			}
		}
		handler(w, nextR)
	})
}

// Handler returns an http.Handler that can be used directly
func (dv *DBViewer) Handler() http.Handler {
	mux := http.NewServeMux()
	dv.Mount(mux)
	return mux
}

// ConnectionInfo represents database connection information
type ConnectionInfo struct {
	Driver   string `json:"driver"`
	Database string `json:"database"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
}

// handleConnectionInfo returns database connection information
func (dv *DBViewer) handleConnectionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current database name using adapter
	dbName, err := dv.adapter.GetCurrentDatabase(dv.db)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to get database name: %v", err), http.StatusInternalServerError)
		return
	}

	// Get connection stats
	stats := dv.db.Stats()

	// Get driver name from adapter
	driverName := "Unknown"
	if dv.adapter != nil {
		switch dv.adapter.GetDriverName() {
		case "postgres":
			driverName = "PostgreSQL"
		case "mysql":
			driverName = "MySQL"
		case "sqlite3":
			driverName = "SQLite"
		default:
			driverName = dv.adapter.GetDriverName()
		}
	}

	info := ConnectionInfo{
		Driver:   driverName,
		Database: dbName,
		Host:     "Connected",
		Port:     "",
		User:     fmt.Sprintf("Max Open Connections: %d, In Use: %d", stats.MaxOpenConnections, stats.InUse),
	}

	respondJSON(w, info)
}

// handleDatabases returns list of databases or schemas
func (dv *DBViewer) handleDatabases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Use adapter to get databases/schemas
	databases, err := dv.adapter.GetDatabases(dv.db)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to query databases: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, databases)
}

// handleTables returns list of tables for a schema or database
func (dv *DBViewer) handleTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dbOrSchema := r.URL.Query().Get("database")
	if dbOrSchema == "" {
		dbOrSchema = "public"
	}

	// Validate name (prevent SQL injection)
	if !isValidTableName(dbOrSchema) {
		respondError(w, "invalid database/schema name", http.StatusBadRequest)
		return
	}

	// Get DB connection (either from DSN template or default)
	db, closeFunc, err := dv.getDB(dbOrSchema)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to connect to database: %v", err), http.StatusInternalServerError)
		return
	}
	if closeFunc != nil {
		defer closeFunc()
	}

	// Use adapter to get tables
	tables, err := dv.adapter.GetTables(db, dbOrSchema)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to query tables: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, tables)
}

// handleTableData returns data from a specific table
func (dv *DBViewer) handleTableData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		respondError(w, "table parameter is required", http.StatusBadRequest)
		return
	}

	dbOrSchema := r.URL.Query().Get("database")
	if dbOrSchema == "" {
		dbOrSchema = "public"
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Sanitize names (basic protection)
	if !isValidTableName(dbOrSchema) || !isValidTableName(tableName) {
		respondError(w, "invalid database/schema or table name", http.StatusBadRequest)
		return
	}

	// Get DB connection (either from DSN template or default)
	db, closeFunc, err := dv.getDB(dbOrSchema)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to connect to database: %v", err), http.StatusInternalServerError)
		return
	}
	if closeFunc != nil {
		defer closeFunc()
	}

	// Get table columns for smart ordering
	columns, err := dv.adapter.GetTableColumns(db, dbOrSchema, tableName)
	if err != nil {
		// If we can't get columns, fall back to simple query without ordering
		query := dv.adapter.BuildTableDataQuery(dbOrSchema, tableName, limit)
		result, err := executeQueryOnDB(db, query)
		if err != nil {
			respondError(w, fmt.Sprintf("Failed to query table: %v", err), http.StatusInternalServerError)
			return
		}
		respondJSON(w, result)
		return
	}

	// Build query with smart ordering (prioritizes id, updated_at, update_time, etc. DESC)
	query := dv.adapter.BuildTableDataQueryWithOrder(dbOrSchema, tableName, columns, limit)

	// Execute query using the selected DB connection
	result, err := executeQueryOnDB(db, query)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to query table: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, result)
}

// handleQuery executes a custom SQL query
func (dv *DBViewer) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SQL string `json:"sql"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SQL == "" {
		respondError(w, "sql is required", http.StatusBadRequest)
		return
	}

	// Only allow SELECT queries for safety
	trimmedSQL := strings.TrimSpace(strings.ToUpper(req.SQL))
	if !strings.HasPrefix(trimmedSQL, "SELECT") && !strings.HasPrefix(trimmedSQL, "WITH") {
		respondError(w, "Only SELECT queries are allowed", http.StatusBadRequest)
		return
	}

	result, err := dv.executeQuery(req.SQL)
	if err != nil {
		respondError(w, fmt.Sprintf("Query failed: %v", err), http.StatusBadRequest)
		return
	}

	respondJSON(w, result)
}

// handleTableSchema returns the schema/structure of a table
func (dv *DBViewer) handleTableSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		respondError(w, "table parameter is required", http.StatusBadRequest)
		return
	}

	dbOrSchema := r.URL.Query().Get("database")
	if dbOrSchema == "" {
		dbOrSchema = "public"
	}

	// Sanitize names
	if !isValidTableName(dbOrSchema) || !isValidTableName(tableName) {
		respondError(w, "invalid database/schema or table name", http.StatusBadRequest)
		return
	}

	// Get DB connection
	db, closeFunc, err := dv.getDB(dbOrSchema)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to connect to database: %v", err), http.StatusInternalServerError)
		return
	}
	if closeFunc != nil {
		defer closeFunc()
	}

	// Get table schema using adapter
	schema, err := dv.adapter.GetTableSchema(db, dbOrSchema, tableName)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to get table schema: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{"columns": schema})
}

// handleTableIndexes returns the indexes of a table
func (dv *DBViewer) handleTableIndexes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		respondError(w, "table parameter is required", http.StatusBadRequest)
		return
	}

	dbOrSchema := r.URL.Query().Get("database")
	if dbOrSchema == "" {
		dbOrSchema = "public"
	}

	// Sanitize names
	if !isValidTableName(dbOrSchema) || !isValidTableName(tableName) {
		respondError(w, "invalid database/schema or table name", http.StatusBadRequest)
		return
	}

	// Get DB connection
	db, closeFunc, err := dv.getDB(dbOrSchema)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to connect to database: %v", err), http.StatusInternalServerError)
		return
	}
	if closeFunc != nil {
		defer closeFunc()
	}

	// Get table indexes using adapter
	indexes, err := dv.adapter.GetTableIndexes(db, dbOrSchema, tableName)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to get table indexes: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{"indexes": indexes})
}

// handleTableDDL returns the CREATE TABLE statement for a table
func (dv *DBViewer) handleTableDDL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		respondError(w, "table parameter is required", http.StatusBadRequest)
		return
	}

	dbOrSchema := r.URL.Query().Get("database")
	if dbOrSchema == "" {
		dbOrSchema = "public"
	}

	// Sanitize names
	if !isValidTableName(dbOrSchema) || !isValidTableName(tableName) {
		respondError(w, "invalid database/schema or table name", http.StatusBadRequest)
		return
	}

	// Get DB connection
	db, closeFunc, err := dv.getDB(dbOrSchema)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to connect to database: %v", err), http.StatusInternalServerError)
		return
	}
	if closeFunc != nil {
		defer closeFunc()
	}

	// Get table DDL using adapter
	ddl, err := dv.adapter.GetTableDDL(db, dbOrSchema, tableName)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to get table DDL: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{"ddl": ddl})
}

// handleTableStats returns statistics for a table (row count, size, etc.)
func (dv *DBViewer) handleTableStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		respondError(w, "table parameter is required", http.StatusBadRequest)
		return
	}

	dbOrSchema := r.URL.Query().Get("database")
	if dbOrSchema == "" {
		dbOrSchema = "public"
	}

	// Sanitize names
	if !isValidTableName(dbOrSchema) || !isValidTableName(tableName) {
		respondError(w, "invalid database/schema or table name", http.StatusBadRequest)
		return
	}

	// Get DB connection
	db, closeFunc, err := dv.getDB(dbOrSchema)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to connect to database: %v", err), http.StatusInternalServerError)
		return
	}
	if closeFunc != nil {
		defer closeFunc()
	}

	// Get table stats using adapter
	stats, err := dv.adapter.GetTableStats(db, dbOrSchema, tableName)
	if err != nil {
		respondError(w, fmt.Sprintf("Failed to get table statistics: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, stats)
}

// QueryResult represents the result of a SQL query
type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Count   int             `json:"count"`
}

// executeQuery executes a SQL query and returns the result (uses default DB)
func (dv *DBViewer) executeQuery(query string) (*QueryResult, error) {
	return executeQueryOnDB(dv.db, query)
}

// executeQueryOnDB executes a SQL query on a specific DB connection
func executeQueryOnDB(db *sql.DB, query string) (*QueryResult, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result [][]interface{}
	for rows.Next() {
		// Create a slice of interface{}'s to represent each column
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Convert byte slices to strings for JSON serialization
		row := make([]interface{}, len(columns))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &QueryResult{
		Columns: columns,
		Rows:    result,
		Count:   len(result),
	}, nil
}

// getDB returns a database connection for the specified database
// If multi-database mode is enabled (dsnPrefix is set), it creates a temporary connection
// Otherwise, it returns the default connection
// The returned closeFunc should be called when done (can be nil)
func (dv *DBViewer) getDB(dbName string) (*sql.DB, func(), error) {
	// If no DSN prefix, use default connection (single database mode)
	if dv.dsnPrefix == "" {
		return dv.db, nil, nil
	}

	// Create temporary connection to the specified database
	var dsn string

	// Handle PostgreSQL DSN using url.Parse
	if strings.HasPrefix(dv.dsnPrefix, "postgres://") || strings.HasPrefix(dv.dsnPrefix, "postgresql://") {
		u, err := url.Parse(dv.dsnPrefix)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse DSN: %w", err)
		}

		// Set the database name in the path
		u.Path = "/" + dbName
		dsn = u.String()

	} else if strings.Contains(dv.dsnPrefix, "@tcp(") {
		// MySQL DSN: user:pass@tcp(host:3306)/?params
		// Insert database name before the '?'
		lastSlash := strings.LastIndex(dv.dsnPrefix, "/")
		queryStart := strings.Index(dv.dsnPrefix, "?")

		if queryStart > lastSlash {
			// Has query parameters
			dsn = dv.dsnPrefix[:lastSlash+1] + dbName + dv.dsnPrefix[queryStart:]
		} else {
			// No query parameters
			dsn = strings.TrimSuffix(dv.dsnPrefix, "/") + "/" + dbName
		}

	} else {
		// Fallback: simple string manipulation
		lastSlash := strings.LastIndex(dv.dsnPrefix, "/")
		queryStart := strings.Index(dv.dsnPrefix, "?")

		if queryStart > lastSlash {
			dsn = dv.dsnPrefix[:lastSlash+1] + dbName + dv.dsnPrefix[queryStart:]
		} else {
			dsn = strings.TrimSuffix(dv.dsnPrefix, "/") + "/" + dbName
		}
	}

	driverName := dv.adapter.GetDriverName()
	tempDB, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := tempDB.Ping(); err != nil {
		_ = tempDB.Close()
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Return connection with close function
	closeFunc := func() {
		_ = tempDB.Close()
	}

	return tempDB, closeFunc, nil
}

// isValidTableName checks if a table name is valid (basic sanitization)
func isValidTableName(name string) bool {
	// Allow alphanumeric, underscore, and dot (for schema.table)
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '.') {
			return false
		}
	}
	return len(name) > 0
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// respondError sends an error response
func respondError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
