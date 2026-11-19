package database

import (
	"path/filepath"
	"testing"

	"github.com/beego/beego/v2/client/orm"
)

// Upgrading an existing database must add columns introduced later (e.g. users.token_version,
// refresh_tokens.auth_time) with safe defaults, not just create missing tables.
func TestInitAddsMissingColumnsToExistingTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	if err := Init("sqlite", "file:"+path); err != nil {
		t.Fatal(err)
	}
	db, err := orm.GetDB("default")
	if err != nil {
		t.Fatal(err)
	}
	// Recreate a pre-upgrade database: an existing user and no token_version/auth_time columns.
	for _, q := range []string{
		`INSERT INTO users (id, email, name, password_hash, created_at, updated_at) VALUES ('u1', 'old@example.com', '', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
		`ALTER TABLE users DROP COLUMN token_version`,
		`ALTER TABLE refresh_tokens DROP COLUMN auth_time`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	if err := orm.RunSyncdb("default", false, false); err != nil {
		t.Fatal(err)
	}
	var version int64
	if err := db.QueryRow(`SELECT token_version FROM users WHERE id = 'u1'`).Scan(&version); err != nil || version != 0 {
		t.Fatalf("token_version = %d, %v; want 0 for existing rows", version, err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(auth_time) FROM refresh_tokens`).Scan(&n); err != nil {
		t.Fatalf("refresh_tokens.auth_time missing: %v", err)
	}
}
