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

// buildSmartOrderBy generates an ORDER BY clause based on available columns.
//
// Priority (highest first):
//  1. updated_at / update_time / updated_time / updatetime
//  2. created_at / create_time / created_time / createtime
//  3. any auto-increment column (ranked above a plain "id" column so tables
//     whose auto-increment PK is not literally named "id" still order sensibly)
//  4. id
func buildSmartOrderBy(columns []string, autoIncrementColumns []string) string {
	// Create a map for quick lookup (case-insensitive)
	columnMap := make(map[string]string)
	for _, col := range columns {
		columnMap[strings.ToLower(col)] = col
	}

	timePriority := []string{
		"updated_at",
		"update_time",
		"updated_time",
		"updatetime",
		"created_at",
		"create_time",
		"created_time",
		"createtime",
	}
	for _, field := range timePriority {
		if actualCol, exists := columnMap[field]; exists {
			return actualCol + " DESC"
		}
	}

	// Auto-increment columns rank above a plain "id" column.
	for _, col := range autoIncrementColumns {
		if col != "" {
			return col + " DESC"
		}
	}

	if actualCol, exists := columnMap["id"]; exists {
		return actualCol + " DESC"
	}
	return ""
}

// dbAdapter defines the interface for different database types
type dbAdapter interface {
	// GetDatabases returns list of databases (for multi-database mode)
	GetDatabases(db *sql.DB) ([]string, error)

	// GetTables returns list of tables in the current database/schema
	GetTables(db *sql.DB, dbName string) ([]string, error)

	// GetTableColumns returns list of column names for a table
	GetTableColumns(db *sql.DB, dbName, tableName string) ([]string, error)

	// GetAutoIncrementColumns returns the names of auto-increment columns in a table.
	GetAutoIncrementColumns(db *sql.DB, dbName, tableName string) ([]string, error)

	// BuildTableDataQuery builds a query to fetch table data
	BuildTableDataQuery(dbName, tableName string, limit int) string

	// BuildTableDataQueryWithOrder builds a query with smart ordering based on
	// available columns. autoIncrementColumns ranks above a plain "id" column.
	BuildTableDataQueryWithOrder(dbName, tableName string, columns []string, autoIncrementColumns []string, limit int) string

	// SupportMultiDatabase returns true if this database type supports multi-database mode
	SupportMultiDatabase() bool

	// GetDriverName returns the SQL driver name
	GetDriverName() string

	// GetCurrentDatabase returns the name of the current database
	GetCurrentDatabase(db *sql.DB) (string, error)

	// GetDatabaseSize returns the on-disk size in bytes of the selected database
	GetDatabaseSize(db *sql.DB, dbName string) (int64, error)

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

// schemaAdapter is implemented by adapters whose databases contain multiple
// schemas (PostgreSQL). Handlers pass table names to such adapters qualified
// as "schema.table"; an unqualified name means the default schema.
type schemaAdapter interface {
	// GetSchemas returns the user schemas the current role can use.
	GetSchemas(db *sql.DB) ([]string, error)
	// GetTablesInSchema returns the base tables of one schema.
	GetTablesInSchema(db *sql.DB, schema string) ([]string, error)
}

const pgDefaultSchema = "public"

// pgSplitTable splits "schema.table" into its parts. Only the first dot
// separates, so a table whose own name contains a dot stays intact when the
// handler qualified it. An unqualified name belongs to the default schema.
func pgSplitTable(name string) (schema, table string) {
	if i := strings.Index(name, "."); i > 0 {
		return name[:i], name[i+1:]
	}
	return pgDefaultSchema, name
}

// pgQuoteIdent quotes a PostgreSQL identifier so mixed-case names and
// schema-qualified names resolve exactly instead of via case folding.
func pgQuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// pgQualifiedName returns the quoted "schema"."table" form of a table name.
func pgQualifiedName(name string) string {
	schema, table := pgSplitTable(name)
	return pgQuoteIdent(schema) + "." + pgQuoteIdent(table)
}

func (p *postgresAdapter) GetSchemas(db *sql.DB) ([]string, error) {
	query := `
		SELECT nspname
		FROM pg_namespace
		WHERE nspname <> 'information_schema'
		  AND nspname NOT LIKE 'pg\_%'
		  AND has_schema_privilege(oid, 'USAGE')
		ORDER BY nspname
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var schemas []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		schemas = append(schemas, name)
	}
	return schemas, rows.Err()
}

func (p *postgresAdapter) GetTables(db *sql.DB, dbName string) ([]string, error) {
	return p.GetTablesInSchema(db, pgDefaultSchema)
}

func (p *postgresAdapter) GetTablesInSchema(db *sql.DB, schema string) ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`
	rows, err := db.Query(query, schema)
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
		WHERE table_schema = $1
		AND table_name = $2
		ORDER BY ordinal_position
	`
	schema, table := pgSplitTable(tableName)
	rows, err := db.Query(query, schema, table)
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
	return fmt.Sprintf("SELECT * FROM %s LIMIT %d", pgQualifiedName(tableName), limit)
}

func (p *postgresAdapter) BuildTableDataQueryWithOrder(dbName, tableName string, columns []string, autoIncrementColumns []string, limit int) string {
	from := pgQualifiedName(tableName)
	orderBy := buildSmartOrderBy(columns, autoIncrementColumns)
	if orderBy == "" {
		return fmt.Sprintf("SELECT * FROM %s LIMIT %d", from, limit)
	}
	return fmt.Sprintf("SELECT * FROM %s ORDER BY %s LIMIT %d", from, orderBy, limit)
}

// GetAutoIncrementColumns returns columns whose default is a sequence
// (SERIAL / BIGSERIAL / SMALLSERIAL — i.e. default starts with nextval(),
// or identity columns created via GENERATED ... AS IDENTITY).
func (p *postgresAdapter) GetAutoIncrementColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2
		  AND (column_default LIKE 'nextval(%' OR is_identity = 'YES')
		ORDER BY ordinal_position
	`
	schema, table := pgSplitTable(tableName)
	rows, err := db.Query(query, schema, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
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

// GetDatabaseSize uses the connection's own database: in multi-database mode
// getDB already connected to the selected one.
func (p *postgresAdapter) GetDatabaseSize(db *sql.DB, dbName string) (int64, error) {
	var size int64
	err := db.QueryRow("SELECT pg_database_size(current_database())").Scan(&size)
	return size, err
}

func (p *postgresAdapter) GetTableSchema(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position
	`
	schema, table := pgSplitTable(tableName)
	rows, err := db.Query(query, schema, table)
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
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1 AND t.relname = $2
		GROUP BY i.relname, ix.indisprimary, ix.indisunique
		ORDER BY i.relname
	`
	schema, table := pgSplitTable(tableName)
	rows, err := db.Query(query, schema, table)
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
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position
	`
	schema, table := pgSplitTable(tableName)
	rows, err := db.Query(colQuery, schema, table)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()

	// Keep the familiar bare name for the default schema; qualify otherwise
	// so the DDL recreates the table in the right schema.
	ddlName := table
	if schema != pgDefaultSchema {
		ddlName = pgQualifiedName(tableName)
	}

	var ddl strings.Builder
	ddl.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", ddlName))

	var colDefs []string
	for rows.Next() {
		var colName, dataType string
		var maxLen sql.NullInt64
		var nullable, defaultVal sql.NullString

		if err := rows.Scan(&colName, &dataType, &maxLen, &nullable, &defaultVal); err != nil {
			return "", err
		}

		// Convert nextval('...') defaults to SERIAL/BIGSERIAL/SMALLSERIAL types
		// so the generated DDL is self-contained and can be executed directly.
		isSerial := false
		if defaultVal.Valid && strings.HasPrefix(defaultVal.String, "nextval(") {
			switch dataType {
			case "integer":
				dataType = "serial"
				isSerial = true
			case "bigint":
				dataType = "bigserial"
				isSerial = true
			case "smallint":
				dataType = "smallserial"
				isSerial = true
			}
		}

		colDef := fmt.Sprintf("  %s %s", colName, dataType)
		if !isSerial && maxLen.Valid {
			colDef += fmt.Sprintf("(%d)", maxLen.Int64)
		}
		if !isSerial && nullable.String == "NO" {
			colDef += " NOT NULL"
		}
		if !isSerial && defaultVal.Valid && defaultVal.String != "" {
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
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1 AND t.relname = $2 AND NOT ix.indisprimary
		GROUP BY i.relname, ix.indisprimary, ix.indisunique
		ORDER BY i.relname
	`
	indexRows, err := db.Query(indexQuery, schema, table)
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
				ddl.WriteString(fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s);", indexName, ddlName, cols))
			} else {
				ddl.WriteString(fmt.Sprintf("CREATE INDEX %s ON %s (%s);", indexName, ddlName, cols))
			}
		}
	}

	return ddl.String(), nil
}

func (p *postgresAdapter) GetTableStats(db *sql.DB, dbName, tableName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Resolve the table by schema + name to its oid, so same-named tables in
	// other schemas can't leak into the stats.
	schema, table := pgSplitTable(tableName)
	var rowCount int64
	var tableSize, indexSize, totalSize string
	err := db.QueryRow(`
		SELECT
			c.reltuples::bigint,
			pg_size_pretty(pg_table_size(c.oid)),
			pg_size_pretty(pg_indexes_size(c.oid)),
			pg_size_pretty(pg_total_relation_size(c.oid))
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2
	`, schema, table).Scan(&rowCount, &tableSize, &indexSize, &totalSize)
	if err != nil {
		return nil, err
	}

	// If estimate is invalid (< 0, e.g. never analyzed), use exact count
	if rowCount < 0 {
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", pgQualifiedName(tableName))
		err = db.QueryRow(query).Scan(&rowCount)
		if err != nil {
			return nil, err
		}
	}
	stats["row_count"] = rowCount
	stats["table_size"] = tableSize
	stats["index_size"] = indexSize
	stats["total_size"] = totalSize

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

func (m *mysqlAdapter) BuildTableDataQueryWithOrder(dbName, tableName string, columns []string, autoIncrementColumns []string, limit int) string {
	orderBy := buildSmartOrderBy(columns, autoIncrementColumns)
	if orderBy == "" {
		return m.BuildTableDataQuery(dbName, tableName, limit)
	}
	if dbName != "" {
		return fmt.Sprintf("SELECT * FROM %s.%s ORDER BY %s LIMIT %d", dbName, tableName, orderBy, limit)
	}
	return fmt.Sprintf("SELECT * FROM %s ORDER BY %s LIMIT %d", tableName, orderBy, limit)
}

// GetAutoIncrementColumns returns columns marked AUTO_INCREMENT in MySQL's
// information_schema.columns.extra.
func (m *mysqlAdapter) GetAutoIncrementColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = COALESCE(NULLIF(?, ''), DATABASE())
		  AND table_name = ?
		  AND extra LIKE '%auto_increment%'
		ORDER BY ordinal_position
	`
	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
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

// GetDatabaseSize sums data + index length of every table in the database.
// information_schema values are InnoDB estimates, like SHOW TABLE STATUS.
func (m *mysqlAdapter) GetDatabaseSize(db *sql.DB, dbName string) (int64, error) {
	var size int64
	err := db.QueryRow(`
		SELECT CAST(COALESCE(SUM(data_length + index_length), 0) AS SIGNED)
		FROM information_schema.tables
		WHERE table_schema = COALESCE(NULLIF(?, ''), DATABASE())
	`, dbName).Scan(&size)
	return size, err
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

func (s *sqliteAdapter) BuildTableDataQueryWithOrder(dbName, tableName string, columns []string, autoIncrementColumns []string, limit int) string {
	orderBy := buildSmartOrderBy(columns, autoIncrementColumns)
	if orderBy == "" {
		return fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)
	}
	return fmt.Sprintf("SELECT * FROM %s ORDER BY %s LIMIT %d", tableName, orderBy, limit)
}

// GetAutoIncrementColumns returns auto-increment columns in a SQLite table.
// In SQLite, a column declared as INTEGER PRIMARY KEY is an alias for ROWID
// and auto-increments; this covers the common case without parsing the
// CREATE TABLE statement for the explicit AUTOINCREMENT keyword.
func (s *sqliteAdapter) GetAutoIncrementColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA table_info('%s')", strings.ReplaceAll(tableName, "'", "''"))
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cols []string
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		if pk == 1 && strings.EqualFold(strings.TrimSpace(typ), "INTEGER") {
			cols = append(cols, name)
		}
	}
	return cols, rows.Err()
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

// GetDatabaseSize returns the size of the main database file (page_count *
// page_size). A pending WAL file is not included.
func (s *sqliteAdapter) GetDatabaseSize(db *sql.DB, dbName string) (int64, error) {
	var size int64
	err := db.QueryRow("SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()").Scan(&size)
	return size, err
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

// GetAutoIncrementColumns returns the auto-increment columns in a table
func (ta *TestAdapter) GetAutoIncrementColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	return ta.adapter.GetAutoIncrementColumns(db, dbName, tableName)
}

// BuildTableDataQueryWithOrder builds a query with smart ordering
func (ta *TestAdapter) BuildTableDataQueryWithOrder(dbName, tableName string, columns []string, autoIncrementColumns []string, limit int) string {
	return ta.adapter.BuildTableDataQueryWithOrder(dbName, tableName, columns, autoIncrementColumns, limit)
}

// GetTableSchema returns detailed schema information for a table
func (ta *TestAdapter) GetTableSchema(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error) {
	return ta.adapter.GetTableSchema(db, dbName, tableName)
}

// GetTableIndexes returns index information for a table
func (ta *TestAdapter) GetTableIndexes(db *sql.DB, dbName, tableName string) ([]IndexInfo, error) {
	return ta.adapter.GetTableIndexes(db, dbName, tableName)
}
