package cli_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
)

// TestDoctor_DB_Wiring tests that the database and FTS5 schemas are applied and verified.
func TestDoctor_DB_Wiring(t *testing.T) {
	ctx := context.Background()

	// Open a temporary SQLite DB via crawlkit/store
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_doctor_wiring.db")

	s, err := store.Open(ctx, store.Options{
		Path: dbPath,
	})
	if err != nil {
		t.Fatalf("failed to open crawlkit store: %v", err)
	}
	defer s.Close()

	db := s.DB()

	// Verify schema migration runs cleanly
	if err := schema.Migrate(db); err != nil {
		t.Fatalf("schema.Migrate failed: %v", err)
	}

	// Verify FTS virtual tables exist
	var ftsCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('messages_fts', 'posts_fts', 'saved_posts_fts')").Scan(&ftsCount)
	if err != nil {
		t.Fatalf("querying sqlite_master failed: %v", err)
	}

	if ftsCount != 3 {
		t.Errorf("expected 3 FTS5 virtual tables to exist, found %d", ftsCount)
	}
}

// Note: Live-network Voyager calls, keyring token resolution on non-darwin platforms,
// and check-update's live GitHub API calls are skipped to avoid non-deterministic network failures in unit tests.
