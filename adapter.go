package sqlview

import (
	"database/sql"
	"fmt"
	"strings"

	// Import database drivers
	// Note: DBViewer itself does not import any drivers to avoid forcing dependencies.
	// Import the drivers you need in your main package:
	//
	// PostgreSQL:
	//   _ "github.com/lib/pq"
	//
	// MySQL:
	//   _ "github.com/go-sql-driver/mysql"
	//
	// SQLite (choose one):
	//   _ "github.com/mattn/go-sqlite3"      // CGO-based (faster)
	//   _ "modernc.org/sqlite"               // Pure Go (no CGO)
)

// buildSmartOrderBy generates ORDER BY clause based on available columns
// Priority order: id DESC, updated_at DESC, update_time DESC, created_at DESC, create_time DESC
func buildSmartOrderBy(columns []string) string {
	// Create a map for quick lookup (case-insensitive)
	columnMap := make(map[string]string)
	for _, col := range columns {
		columnMap[strings.ToLower(col)] = col
	}

	// Priority list of column names to order by (descending)
	orderPriority := []string{
		"id",
		"updated_at",
		"update_time",
		"updated_time",
		"updatetime",
		"created_at",
		"create_time",
		"created_time",
		"createtime",
	}

	var orderFields []string
	for _, field := range orderPriority {
		if actualCol, exists := columnMap[field]; exists {
			orderFields = append(orderFields, actualCol+" DESC")
			// Only use the first matching field
			break
		}
	}

	if len(orderFields) == 0 {
		return ""
	}
	return strings.Join(orderFields, ", ")
}

// dbAdapter defines the interface for different database types
type dbAdapter interface {
	// GetDatabases returns list of databases (for multi-database mode)
	GetDatabases(db *sql.DB) ([]string, error)

	// GetTables returns list of tables in the current database/schema
	GetTables(db *sql.DB, dbName string) ([]string, error)

	// GetTableColumns returns list of column names for a table
	GetTableColumns(db *sql.DB, dbName, tableName string) ([]string, error)

	// BuildTableDataQuery builds a query to fetch table data
	BuildTableDataQuery(dbName, tableName string, limit int) string

	// BuildTableDataQueryWithOrder builds a query with smart ordering based on available columns
	BuildTableDataQueryWithOrder(dbName, tableName string, columns []string, limit int) string

	// SupportMultiDatabase returns true if this database type supports multi-database mode
	SupportMultiDatabase() bool

	// GetDriverName returns the SQL driver name
	GetDriverName() string

	// GetCurrentDatabase returns the name of the current database
	GetCurrentDatabase(db *sql.DB) (string, error)

	// GetTableSchema returns the schema/structure of a table
	GetTableSchema(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error)

	// GetTableIndexes returns the indexes of a table
	GetTableIndexes(db *sql.DB, dbName, tableName string) ([]IndexInfo, error)

	// GetTableDDL returns the CREATE TABLE statement for a table
	GetTableDDL(db *sql.DB, dbName, tableName string) (string, error)

	// GetTableStats returns statistics for a table (row count, size, etc.)
	GetTableStats(db *sql.DB, dbName, tableName string) (map[string]interface{}, error)
}

// ColumnInfo represents information about a table column
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default"`
}

// IndexInfo represents information about a table index
type IndexInfo struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

// postgresAdapter implements dbAdapter for PostgreSQL
type postgresAdapter struct{}

func (p *postgresAdapter) GetDatabases(db *sql.DB) ([]string, error) {
	query := `
		SELECT datname
		FROM pg_database
		WHERE datistemplate = false
		ORDER BY datname
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		databases = append(databases, name)
	}
	return databases, nil
}

func (p *postgresAdapter) GetTables(db *sql.DB, dbName string) ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, nil
}

func (p *postgresAdapter) GetTableColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public'
		AND table_name = $1
		ORDER BY ordinal_position
	`
	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, nil
}

func (p *postgresAdapter) BuildTableDataQuery(dbName, tableName string, limit int) string {
	return fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)
}

func (p *postgresAdapter) BuildTableDataQueryWithOrder(dbName, tableName string, columns []string, limit int) string {
	orderBy := buildSmartOrderBy(columns)
	if orderBy == "" {
		return fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)
	}
	return fmt.Sprintf("SELECT * FROM %s ORDER BY %s LIMIT %d", tableName, orderBy, limit)
}

func (p *postgresAdapter) SupportMultiDatabase() bool {
	return true
}

func (p *postgresAdapter) GetDriverName() string {
	return "postgres"
}

func (p *postgresAdapter) GetCurrentDatabase(db *sql.DB) (string, error) {
	var dbName string
	err := db.QueryRow("SELECT current_database()").Scan(&dbName)
	return dbName, err
}

func (p *postgresAdapter) GetTableSchema(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`
	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullable, defaultVal sql.NullString
		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal); err != nil {
			return nil, err
		}
		col.Nullable = nullable.String == "YES"
		col.Default = defaultVal.String
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (p *postgresAdapter) GetTableIndexes(db *sql.DB, dbName, tableName string) ([]IndexInfo, error) {
	query := `
		SELECT
			i.relname as index_name,
			CASE WHEN ix.indisprimary THEN 'PRIMARY KEY' ELSE 'INDEX' END as index_type,
			array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) as column_names,
			ix.indisunique as is_unique
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relname = $1
		GROUP BY i.relname, ix.indisprimary, ix.indisunique
		ORDER BY i.relname
	`
	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var indexes []IndexInfo
	for rows.Next() {
		var idx IndexInfo
		var cols string
		if err := rows.Scan(&idx.Name, &idx.Type, &cols, &idx.Unique); err != nil {
			return nil, err
		}
		// Parse array format: {col1,col2}
		cols = strings.Trim(cols, "{}")
		if cols != "" {
			idx.Columns = strings.Split(cols, ",")
		}
		indexes = append(indexes, idx)
	}
	return indexes, rows.Err()
}

func (p *postgresAdapter) GetTableDDL(db *sql.DB, dbName, tableName string) (string, error) {
	// Get column definitions
	colQuery := `
		SELECT
			column_name,
			data_type,
			character_maximum_length,
			is_nullable,
			column_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`
	rows, err := db.Query(colQuery, tableName)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()

	var ddl strings.Builder
	ddl.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", tableName))

	var colDefs []string
	for rows.Next() {
		var colName, dataType string
		var maxLen sql.NullInt64
		var nullable, defaultVal sql.NullString

		if err := rows.Scan(&colName, &dataType, &maxLen, &nullable, &defaultVal); err != nil {
			return "", err
		}

		colDef := fmt.Sprintf("  %s %s", colName, dataType)
		if maxLen.Valid {
			colDef += fmt.Sprintf("(%d)", maxLen.Int64)
		}
		if nullable.String == "NO" {
			colDef += " NOT NULL"
		}
		if defaultVal.Valid && defaultVal.String != "" {
			colDef += fmt.Sprintf(" DEFAULT %s", defaultVal.String)
		}
		colDefs = append(colDefs, colDef)
	}

	ddl.WriteString(strings.Join(colDefs, ",\n"))
	ddl.WriteString("\n);")

	// Add index creation statements
	indexQuery := `
		SELECT
			i.relname as index_name,
			ix.indisprimary as is_primary,
			ix.indisunique as is_unique,
			array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) as column_names
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relname = $1 AND NOT ix.indisprimary
		GROUP BY i.relname, ix.indisprimary, ix.indisunique
		ORDER BY i.relname
	`
	indexRows, err := db.Query(indexQuery, tableName)
	if err == nil {
		defer func() { _ = indexRows.Close() }()

		for indexRows.Next() {
			var indexName string
			var isPrimary, isUnique bool
			var cols string

			if err := indexRows.Scan(&indexName, &isPrimary, &isUnique, &cols); err != nil {
				continue
			}

			// Parse array format: {col1,col2}
			cols = strings.Trim(cols, "{}")
			if cols == "" {
				continue
			}

			ddl.WriteString("\n\n")
			if isUnique {
				ddl.WriteString(fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s);", indexName, tableName, cols))
			} else {
				ddl.WriteString(fmt.Sprintf("CREATE INDEX %s ON %s (%s);", indexName, tableName, cols))
			}
		}
	}

	return ddl.String(), nil
}

func (p *postgresAdapter) GetTableStats(db *sql.DB, dbName, tableName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get row count (estimate from pg_class, fallback to exact count)
	var rowCount int64
	err := db.QueryRow(`
		SELECT reltuples::bigint FROM pg_class WHERE relname = $1
	`, tableName).Scan(&rowCount)
	if err != nil {
		return nil, err
	}

	// If estimate is invalid (< 0), use exact count
	if rowCount < 0 {
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
		err = db.QueryRow(query).Scan(&rowCount)
		if err != nil {
			return nil, err
		}
	}
	stats["row_count"] = rowCount

	// Get table size
	var tableSize, indexSize, totalSize string
	err = db.QueryRow(`
		SELECT
			pg_size_pretty(pg_table_size($1)) as table_size,
			pg_size_pretty(pg_indexes_size($1)) as index_size,
			pg_size_pretty(pg_total_relation_size($1)) as total_size
	`, tableName).Scan(&tableSize, &indexSize, &totalSize)
	if err == nil {
		stats["table_size"] = tableSize
		stats["index_size"] = indexSize
		stats["total_size"] = totalSize
	}

	return stats, nil
}

// mysqlAdapter implements dbAdapter for MySQL
type mysqlAdapter struct{}

func (m *mysqlAdapter) GetDatabases(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		// Skip system databases
		if name != "information_schema" && name != "mysql" && name != "performance_schema" && name != "sys" {
			databases = append(databases, name)
		}
	}
	return databases, nil
}

func (m *mysqlAdapter) GetTables(db *sql.DB, dbName string) ([]string, error) {
	// Switch to the specified database if provided
	if dbName != "" {
		if _, err := db.Exec("USE " + dbName); err != nil {
			return nil, err
		}
	}

	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, nil
}

func (m *mysqlAdapter) GetTableColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	var query string
	if dbName != "" {
		query = fmt.Sprintf("SHOW COLUMNS FROM %s.%s", dbName, tableName)
	} else {
		query = fmt.Sprintf("SHOW COLUMNS FROM %s", tableName)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []string
	for rows.Next() {
		var field, typ, null, key, def, extra sql.NullString
		if err := rows.Scan(&field, &typ, &null, &key, &def, &extra); err != nil {
			return nil, err
		}
		columns = append(columns, field.String)
	}
	return columns, nil
}

func (m *mysqlAdapter) BuildTableDataQuery(dbName, tableName string, limit int) string {
	if dbName != "" {
		return fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d", dbName, tableName, limit)
	}
	return fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)
}

func (m *mysqlAdapter) BuildTableDataQueryWithOrder(dbName, tableName string, columns []string, limit int) string {
	orderBy := buildSmartOrderBy(columns)
	if orderBy == "" {
		return m.BuildTableDataQuery(dbName, tableName, limit)
	}
	if dbName != "" {
		return fmt.Sprintf("SELECT * FROM %s.%s ORDER BY %s LIMIT %d", dbName, tableName, orderBy, limit)
	}
	return fmt.Sprintf("SELECT * FROM %s ORDER BY %s LIMIT %d", tableName, orderBy, limit)
}

func (m *mysqlAdapter) SupportMultiDatabase() bool {
	return true
}

func (m *mysqlAdapter) GetDriverName() string {
	return "mysql"
}

func (m *mysqlAdapter) GetCurrentDatabase(db *sql.DB) (string, error) {
	var dbName string
	err := db.QueryRow("SELECT DATABASE()").Scan(&dbName)
	return dbName, err
}

func (m *mysqlAdapter) GetTableSchema(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error) {
	query := "SHOW FULL COLUMNS FROM "
	if dbName != "" {
		query += dbName + "."
	}
	query += tableName

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []ColumnInfo
	for rows.Next() {
		var field, typ, collation, null, key, extra string
		var defaultVal, privileges, comment sql.NullString

		if err := rows.Scan(&field, &typ, &collation, &null, &key, &defaultVal, &extra, &privileges, &comment); err != nil {
			return nil, err
		}

		columns = append(columns, ColumnInfo{
			Name:     field,
			Type:     typ,
			Nullable: null == "YES",
			Default:  defaultVal.String,
		})
	}
	return columns, rows.Err()
}

func (m *mysqlAdapter) GetTableIndexes(db *sql.DB, dbName, tableName string) ([]IndexInfo, error) {
	query := "SHOW INDEX FROM "
	if dbName != "" {
		query += dbName + "."
	}
	query += tableName

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	indexMap := make(map[string]*IndexInfo)
	for rows.Next() {
		var table, nonUnique, keyName, seqInIndex, columnName, collation, cardinality, subPart, packed, null, indexType, comment, indexComment string
		var visible sql.NullString

		if err := rows.Scan(&table, &nonUnique, &keyName, &seqInIndex, &columnName, &collation, &cardinality, &subPart, &packed, &null, &indexType, &comment, &indexComment, &visible); err != nil {
			return nil, err
		}

		if idx, exists := indexMap[keyName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			idxType := "INDEX"
			if keyName == "PRIMARY" {
				idxType = "PRIMARY KEY"
			}
			indexMap[keyName] = &IndexInfo{
				Name:    keyName,
				Type:    idxType,
				Columns: []string{columnName},
				Unique:  nonUnique == "0",
			}
		}
	}

	var indexes []IndexInfo
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}
	return indexes, rows.Err()
}

func (m *mysqlAdapter) GetTableDDL(db *sql.DB, dbName, tableName string) (string, error) {
	query := "SHOW CREATE TABLE "
	if dbName != "" {
		query += dbName + "."
	}
	query += tableName

	var table, createTable string
	err := db.QueryRow(query).Scan(&table, &createTable)
	if err != nil {
		return "", err
	}
	return createTable, nil
}

func (m *mysqlAdapter) GetTableStats(db *sql.DB, dbName, tableName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	query := `
		SELECT table_rows, data_length, index_length, data_length + index_length as total_length
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
	`
	var rowCount, dataLen, indexLen, totalLen int64
	err := db.QueryRow(query, tableName).Scan(&rowCount, &dataLen, &indexLen, &totalLen)
	if err != nil {
		return nil, err
	}

	stats["row_count"] = rowCount
	stats["table_size"] = fmt.Sprintf("%.2f MB", float64(dataLen)/1024/1024)
	stats["index_size"] = fmt.Sprintf("%.2f MB", float64(indexLen)/1024/1024)
	stats["total_size"] = fmt.Sprintf("%.2f MB", float64(totalLen)/1024/1024)

	return stats, nil
}

// sqliteAdapter implements dbAdapter for SQLite
type sqliteAdapter struct{}

func (s *sqliteAdapter) GetDatabases(db *sql.DB) ([]string, error) {
	// SQLite doesn't have multiple databases in one connection
	return []string{"main"}, nil
}

func (s *sqliteAdapter) GetTables(db *sql.DB, dbName string) ([]string, error) {
	query := `
		SELECT name
		FROM sqlite_master
		WHERE type='table'
		AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, nil
}

func (s *sqliteAdapter) GetTableColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA table_info('%s')", strings.ReplaceAll(tableName, "'", "''"))
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, nil
}

func (s *sqliteAdapter) BuildTableDataQuery(dbName, tableName string, limit int) string {
	return fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)
}

func (s *sqliteAdapter) BuildTableDataQueryWithOrder(dbName, tableName string, columns []string, limit int) string {
	orderBy := buildSmartOrderBy(columns)
	if orderBy == "" {
		return fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)
	}
	return fmt.Sprintf("SELECT * FROM %s ORDER BY %s LIMIT %d", tableName, orderBy, limit)
}

func (s *sqliteAdapter) SupportMultiDatabase() bool {
	return false
}

func (s *sqliteAdapter) GetDriverName() string {
	return "sqlite3"
}

func (s *sqliteAdapter) GetCurrentDatabase(db *sql.DB) (string, error) {
	// SQLite doesn't have a database name concept, use "main"
	return "main", nil
}

func (s *sqliteAdapter) GetTableSchema(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error) {
	query := fmt.Sprintf("PRAGMA table_info('%s')", strings.ReplaceAll(tableName, "'", "''"))
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []ColumnInfo
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}

		columns = append(columns, ColumnInfo{
			Name:     name,
			Type:     typ,
			Nullable: notnull == 0,
			Default:  dfltValue.String,
		})
	}
	return columns, rows.Err()
}

func (s *sqliteAdapter) GetTableIndexes(db *sql.DB, dbName, tableName string) ([]IndexInfo, error) {
	query := fmt.Sprintf("PRAGMA index_list('%s')", strings.ReplaceAll(tableName, "'", "''"))
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	// First, collect all index info
	type indexMetadata struct {
		name    string
		unique  string
		origin  string
		partial string
	}
	var indexMetas []indexMetadata
	for rows.Next() {
		var seq int
		var name, unique, origin, partial string

		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			_ = rows.Close()
			return nil, err
		}
		indexMetas = append(indexMetas, indexMetadata{
			name:    name,
			unique:  unique,
			origin:  origin,
			partial: partial,
		})
	}
	_ = rows.Close()

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Now query column info for each index
	var indexes []IndexInfo
	for _, meta := range indexMetas {
		// Get columns for this index
		colQuery := fmt.Sprintf("PRAGMA index_info('%s')", strings.ReplaceAll(meta.name, "'", "''"))
		colRows, err := db.Query(colQuery)
		if err != nil {
			continue
		}

		var cols []string
		for colRows.Next() {
			var seqno int
			var cid sql.NullInt64
			var colName sql.NullString
			if err := colRows.Scan(&seqno, &cid, &colName); err != nil {
				continue
			}
			if colName.Valid {
				cols = append(cols, colName.String)
			}
		}
		_ = colRows.Close()

		idxType := "INDEX"
		if meta.origin == "pk" {
			idxType = "PRIMARY KEY"
		}

		indexes = append(indexes, IndexInfo{
			Name:    meta.name,
			Type:    idxType,
			Columns: cols,
			Unique:  meta.unique != "0",
		})
	}
	return indexes, nil
}

func (s *sqliteAdapter) GetTableDDL(db *sql.DB, dbName, tableName string) (string, error) {
	var ddl string
	query := "SELECT sql FROM sqlite_master WHERE type='table' AND name=?"
	err := db.QueryRow(query, tableName).Scan(&ddl)
	if err != nil {
		return "", err
	}

	// Add index creation statements
	indexQuery := "SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name=? AND sql IS NOT NULL"
	indexRows, err := db.Query(indexQuery, tableName)
	if err == nil {
		defer func() { _ = indexRows.Close() }()

		var indexDDLs []string
		for indexRows.Next() {
			var indexSQL string
			if err := indexRows.Scan(&indexSQL); err == nil && indexSQL != "" {
				indexDDLs = append(indexDDLs, indexSQL)
			}
		}

		if len(indexDDLs) > 0 {
			ddl += ";\n\n" + strings.Join(indexDDLs, ";\n")
		}
	}

	return ddl, nil
}

func (s *sqliteAdapter) GetTableStats(db *sql.DB, dbName, tableName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get row count
	var rowCount int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err := db.QueryRow(query).Scan(&rowCount)
	if err != nil {
		return nil, err
	}
	stats["row_count"] = rowCount

	// SQLite doesn't have easy table size queries, so we'll skip those
	return stats, nil
}

// getAdapter returns the appropriate adapter for the given database type
func getAdapter(dbType string) dbAdapter {
	switch dbType {
	case "postgres", "postgresql", "pgx":
		return &postgresAdapter{}
	case "mysql":
		return &mysqlAdapter{}
	case "sqlite", "sqlite3":
		return &sqliteAdapter{}
	default:
		return &postgresAdapter{} // default to postgres
	}
}

// TestAdapter provides access to database adapter methods for testing purposes.
// This allows external test packages to test adapter functionality.
type TestAdapter struct {
	adapter dbAdapter
}

// NewTestAdapter creates a TestAdapter for the given database type.
// Supported types: "sqlite", "postgres", "mysql"
func NewTestAdapter(dbType string) *TestAdapter {
	return &TestAdapter{
		adapter: getAdapter(dbType),
	}
}

// GetDatabases returns list of databases
func (ta *TestAdapter) GetDatabases(db *sql.DB) ([]string, error) {
	return ta.adapter.GetDatabases(db)
}

// GetTables returns list of tables in a database
func (ta *TestAdapter) GetTables(db *sql.DB, dbName string) ([]string, error) {
	return ta.adapter.GetTables(db, dbName)
}

// GetTableColumns returns list of column names for a table
func (ta *TestAdapter) GetTableColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	return ta.adapter.GetTableColumns(db, dbName, tableName)
}

// GetTableSchema returns detailed schema information for a table
func (ta *TestAdapter) GetTableSchema(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error) {
	return ta.adapter.GetTableSchema(db, dbName, tableName)
}

// GetTableIndexes returns index information for a table
func (ta *TestAdapter) GetTableIndexes(db *sql.DB, dbName, tableName string) ([]IndexInfo, error) {
	return ta.adapter.GetTableIndexes(db, dbName, tableName)
}
