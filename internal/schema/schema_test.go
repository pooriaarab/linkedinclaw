package schema_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
)

func TestMigrate(t *testing.T) {
	ctx := context.Background()

	// 1. Open a temporary SQLite DB via crawlkit/store
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	s, err := store.Open(ctx, store.Options{
		Path: dbPath,
	})
	if err != nil {
		t.Fatalf("failed to open crawlkit store: %v", err)
	}
	defer s.Close()

	db := s.DB()

	// 2. Call schema.Migrate
	if err := schema.Migrate(db); err != nil {
		t.Fatalf("schema.Migrate failed: %v", err)
	}

	// 3. Query sqlite_master to verify that all 7 real tables + sync_state exist
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatalf("failed to query sqlite_master: %v", err)
	}
	defer rows.Close()

	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan table name: %v", err)
		}
		tables[name] = true
	}

	expectedTables := []string{
		"profile",
		"connections",
		"conversations",
		"messages",
		"posts",
		"saved_posts",
		"companies_followed",
		"sync_state",
	}

	for _, tbl := range expectedTables {
		if !tables[tbl] {
			t.Errorf("expected table %q to exist, but it was not found. Found tables: %v", tbl, tables)
		}
	}
}
