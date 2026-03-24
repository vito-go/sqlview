# Binary Data Encoding Feature (BYTEA Support)

## Overview

SQLView now supports intelligent encoding of binary data columns (such as PostgreSQL's `BYTEA` type). This feature provides:

1. **Smart default encoding**: Automatically chooses encoding based on column names
2. **User-configurable encoding**: Switch between Raw/Hex/Base64 for each column
3. **Persistent preferences**: Encoding choices saved to browser localStorage

## Features

### 1. Automatic Encoding Detection

When displaying binary columns, SQLView automatically detects the data type and applies smart default encoding:

- **Columns containing "key"** (case-insensitive): Default to **Hex** encoding
  - Examples: `encryption_key`, `api_key`, `secret_key`
- **Other binary columns**: Default to **Base64** encoding
  - Examples: `user_avatar`, `file_data`, `image_blob`

### 2. Supported Encodings

Each binary column supports three encoding options:

| Encoding | Description | Use Case |
|----------|-------------|----------|
| **Raw** | Display as plain text | When data is actually text stored as binary |
| **Hex** | Hexadecimal encoding | Keys, hashes, low-level binary data |
| **Base64** | Base64 encoding | Images, files, general binary data |

### 3. User Interface

Binary columns have a dropdown selector in the table header:

```
┌─────────────────┐
│ encryption_key  │
│ [Hex ▼]         │  ← Encoding selector
├─────────────────┤
│ deadbeef        │  ← Hex-encoded data
│ cafebabe        │
└─────────────────┘
```

### 4. Persistent Preferences

Encoding selections are saved to browser localStorage with the key format:
```
encoding:{database}:{table}:{column}
```

Example:
```
encoding:mydb:users:profile_picture = "base64"
encoding:mydb:auth:api_key = "hex"
```

## Backend Implementation

### Database Type Support

The feature automatically recognizes binary types from various databases:

**PostgreSQL:**
- `bytea`

**MySQL:**
- `blob`, `tinyblob`, `mediumblob`, `longblob`
- `binary`, `varbinary`

**SQLite:**
- `blob`

**SQL Server:**
- `varbinary`, `varbinary(max)`, `image`, `bit`

### API Response

The backend returns column type information in the query result:

```json
{
  "columns": ["id", "encryption_key", "user_avatar"],
  "rows": [
    [1, "deadbeef", "SGVsbG8sIFdvcmxkIQ=="],
    [2, "cafebabe", "VGVzdCBkYXRh"]
  ],
  "count": 2,
  "columnTypes": [
    {
      "name": "id",
      "type": "int4",
      "isBinary": false,
      "defaultEncoding": "raw"
    },
    {
      "name": "encryption_key",
      "type": "bytea",
      "isBinary": true,
      "defaultEncoding": "hex"
    },
    {
      "name": "user_avatar",
      "type": "bytea",
      "isBinary": true,
      "defaultEncoding": "base64"
    }
  ]
}
```

## Usage Example

See [examples/example_bytea.go](examples/example_bytea.go) for a complete working example.

```go
package main

import (
    "database/sql"
    _ "github.com/lib/pq"
    "github.com/vito-go/sqlview"
)

func main() {
    db, _ := sql.Open("postgres", "postgres://localhost/mydb")

    // Create viewer
    viewer := sqlview.New(db, "/dbviewer")

    // The viewer will automatically detect BYTEA columns
    // and apply intelligent encoding
    viewer.Mount(http.DefaultServeMux)

    http.ListenAndServe(":8080", nil)
}
```

## Benefits

1. **Better UX**: Users can easily view binary data in the format that makes sense
2. **Smart defaults**: Common patterns (keys, images) get appropriate encoding automatically
3. **Flexibility**: Users can change encoding on-the-fly without re-querying
4. **Persistence**: Preferences are remembered across sessions
5. **Performance**: Re-encoding happens client-side without server round-trips

## Technical Details

### Frontend Encoding Functions

The frontend includes utility functions for encoding conversion:

```javascript
// Convert between encodings
reencodeData(data, fromEncoding, toEncoding)

// Encoding/decoding
hexToBytes(hex)
base64ToBytes(base64)
bytesToHex(bytes)
bytesToBase64(bytes)
```

### Backend Encoding Logic

The backend determines default encoding in `executeQueryOnDB()`:

```go
func executeQueryOnDB(db *sql.DB, query string) (*QueryResult, error) {
    // Get column types
    columnTypes, _ := rows.ColumnTypes()

    // Determine encoding for each column
    for i, col := range columns {
        dbTypeName := columnTypes[i].DatabaseTypeName()
        isBinary := isBinaryType(dbTypeName)

        if isBinary {
            // Check column name for "key"
            if strings.Contains(strings.ToLower(col), "key") {
                defaultEncoding = "hex"
            } else {
                defaultEncoding = "base64"
            }
        }
    }

    // Encode data accordingly
    // ...
}
```

## Compatibility

- ✅ PostgreSQL (tested with BYTEA)
- ✅ MySQL (BLOB types)
- ✅ SQLite (BLOB type)
- ⚠️  Other databases (should work with binary types, not tested)

## Future Enhancements

Potential improvements:

- [ ] Add more encoding options (e.g., URL-safe Base64)
- [ ] Support binary data visualization (image preview)
- [ ] Export binary columns in chosen encoding
- [ ] Global encoding preferences per data type
