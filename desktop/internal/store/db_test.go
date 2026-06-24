package store

import (
	"database/sql"
	"testing"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
