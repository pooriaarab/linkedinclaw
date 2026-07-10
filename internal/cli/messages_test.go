package cli_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/cli"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
)

func TestQueryMessages(t *testing.T) {
	ctx := context.Background()

	// 1. Open a temporary SQLite DB via crawlkit/store
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_messages.db")

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
		VALUES 
			('urn:li:fs_miniProfile:alice', 'Alice', 'Smith', 'Manager', 'Corp A', '2026-07-01T12:00:00Z', 'api'),
			('urn:li:fs_miniProfile:bob', 'Bob', 'Jones', 'Dev', 'Corp B', '2026-07-01T12:00:00Z', 'api');
	`)
	if err != nil {
		t.Fatalf("failed to insert connections: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO conversations (urn, participants, last_activity_at)
		VALUES 
			('urn:li:fs_conversation:c1', 'Alice Smith', '2026-07-01T12:00:00Z'),
			('urn:li:fs_conversation:c2', 'Bob Jones', '2026-07-01T12:00:00Z');
	`)
	if err != nil {
		t.Fatalf("failed to insert conversations: %v", err)
	}

	// Set sent_at timestamps: one very recent, one older
	now := time.Now().UTC()
	recentTimeStr := now.Add(-1 * time.Hour).Format("2006-01-02T15:04:05Z")
	olderTimeStr := now.Add(-10 * time.Hour).Format("2006-01-02T15:04:05Z")

	_, err = db.ExecContext(ctx, `
		INSERT INTO messages (urn, conversation_urn, sender_urn, body, sent_at, source)
		VALUES 
			('urn:li:fs_message:m1', 'urn:li:fs_conversation:c1', 'urn:li:fs_miniProfile:alice', 'Hi from Alice!', ?, 'api'),
			('urn:li:fs_message:m2', 'urn:li:fs_conversation:c2', 'urn:li:fs_miniProfile:bob', 'Hi from Bob!', ?, 'api');
	`, recentTimeStr, olderTimeStr)
	if err != nil {
		t.Fatalf("failed to insert messages: %v", err)
	}

	// 4. Test filtering by Person
	results, err := cli.QueryMessages(db, "Alice", 0)
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 message from Alice, got %d", len(results))
	}
	if results[0].Sender != "Alice Smith" || results[0].Body != "Hi from Alice!" {
		t.Errorf("unexpected Alice message result: %+v", results[0])
	}

	// 5. Test filtering by Time Hours
	// Under 2 hours -> should only return Alice (sent 1h ago), Bob (sent 10h ago) should be excluded
	resultsTime, err := cli.QueryMessages(db, "", 2)
	if err != nil {
		t.Fatalf("failed to query messages by time: %v", err)
	}
	if len(resultsTime) != 1 {
		t.Fatalf("expected 1 recent message, got %d", len(resultsTime))
	}
	if resultsTime[0].Sender != "Alice Smith" {
		t.Errorf("expected recent message to be from Alice, got sender: %q", resultsTime[0].Sender)
	}

	// Under 12 hours -> should return both Alice and Bob
	resultsTimeBoth, err := cli.QueryMessages(db, "", 12)
	if err != nil {
		t.Fatalf("failed to query messages by longer time: %v", err)
	}
	if len(resultsTimeBoth) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resultsTimeBoth))
	}
}
