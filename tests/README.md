# Tests and Examples

This directory contains tests and examples for sqlview with independent go.mod to keep the main module clean from test dependencies.

## Running Tests

```bash
cd tests
go test -v
```

## Test Coverage

- `sqlite_test.go`: SQLite PRAGMA commands test (tests the bug fix for SQLite hanging issue)
- `example_test.go`: Usage examples for the library
