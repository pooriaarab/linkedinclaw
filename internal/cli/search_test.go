package cli_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/cli"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
)

func TestQuery_FTS(t *testing.T) {
	ctx := context.Background()

	// 1. Open a temporary SQLite DB via crawlkit/store
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_search.db")

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

	// 3. Seed some dummy data
	_, err = db.ExecContext(ctx, `
		INSERT INTO connections (urn, first_name, last_name, headline, company, connected_at, source)
		VALUES ('urn:li:fs_miniProfile:111', 'John', 'Doe', 'Engineer', 'Acme', '2026-07-01T12:00:00Z', 'api');
	`)
	if err != nil {
		t.Fatalf("failed to insert connection: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO conversations (urn, participants, last_activity_at)
		VALUES ('urn:li:fs_conversation:1', 'John Doe', '2026-07-01T12:00:00Z');
	`)
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO messages (urn, conversation_urn, sender_urn, body, sent_at, source)
		VALUES ('urn:li:fs_message:1', 'urn:li:fs_conversation:1', 'urn:li:fs_miniProfile:111', 'Hello there, this is a secret query term and message body!', '2026-07-02T10:00:00Z', 'api');
	`)
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO posts (urn, body, posted_at, like_count, comment_count)
		VALUES ('urn:li:fs_post:1', 'Just published an article about query term optimization in databases.', '2026-07-03T14:00:00Z', 5, 2);
	`)
	if err != nil {
		t.Fatalf("failed to insert post: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO saved_posts (urn, author, body, saved_at)
		VALUES ('urn:li:fs_saved_post:1', 'Jane Smith', 'Saved this post about query term for later reference.', '2026-07-04T09:00:00Z');
	`)
	if err != nil {
		t.Fatalf("failed to insert saved_post: %v", err)
	}

	// Rebuild FTS indexes as they are content-backed
	_, err = db.ExecContext(ctx, "INSERT INTO messages_fts(messages_fts) VALUES('rebuild');")
	if err != nil {
		t.Fatalf("failed to rebuild messages_fts: %v", err)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO posts_fts(posts_fts) VALUES('rebuild');")
	if err != nil {
		t.Fatalf("failed to rebuild posts_fts: %v", err)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO saved_posts_fts(saved_posts_fts) VALUES('rebuild');")
	if err != nil {
		t.Fatalf("failed to rebuild saved_posts_fts: %v", err)
	}

	// 4. Run Query and verify matching rows
	results, err := cli.Query(db, "term")
	if err != nil {
		t.Fatalf("cli.Query failed: %v", err)
	}

	// Expect matches in messages first, then posts, then saved_posts
	if len(results) != 3 {
		t.Fatalf("expected 3 search results, got %d", len(results))
	}

	expected := []struct {
		kind string
		urn  string
	}{
		{"message", "urn:li:fs_message:1"},
		{"post", "urn:li:fs_post:1"},
		{"saved_post", "urn:li:fs_saved_post:1"},
	}

	for i, exp := range expected {
		if results[i].Kind != exp.kind {
			t.Errorf("at index %d: expected kind %s, got %s", i, exp.kind, results[i].Kind)
		}
		if results[i].Urn != exp.urn {
			t.Errorf("at index %d: expected urn %s, got %s", i, exp.urn, results[i].Urn)
		}
		if results[i].Snippet == "" {
			t.Errorf("at index %d: snippet was empty", i)
		}
	}
}
