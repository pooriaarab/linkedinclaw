package cli_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/openclaw/crawlkit/control"
	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/cli"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
)

func TestStatus_Logic(t *testing.T) {
	ctx := context.Background()

	// 1. Manifest construction
	manifest := control.NewManifest("linkedinclaw", "linkedinclaw", "linkedinclaw")
	if manifest.ID != "linkedinclaw" {
		t.Errorf("expected ID 'linkedinclaw', got %q", manifest.ID)
	}

	// 2. Open a temporary SQLite DB via crawlkit/store to test QueryCount
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_status.db")

	s, err := store.Open(ctx, store.Options{
		Path: dbPath,
	})
	if err != nil {
		t.Fatalf("failed to open crawlkit store: %v", err)
	}
	defer s.Close()

	db := s.DB()

	// Call schema.Migrate
	if err := schema.Migrate(db); err != nil {
		t.Fatalf("schema.Migrate failed: %v", err)
	}

	// Seed some dummy data
	_, err = db.ExecContext(ctx, `
		INSERT INTO connections (urn, first_name, last_name, headline, company, connected_at, source)
		VALUES ('urn:li:fs_miniProfile:111', 'John', 'Doe', 'Engineer', 'Acme', '2026-07-01T12:00:00Z', 'api');
	`)
	if err != nil {
		t.Fatalf("failed to insert connection: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO connections (urn, first_name, last_name, headline, company, connected_at, source)
		VALUES ('urn:li:fs_miniProfile:222', 'Jane', 'Smith', 'Manager', 'Beta', '2026-07-02T12:00:00Z', 'api');
	`)
	if err != nil {
		t.Fatalf("failed to insert connection: %v", err)
	}

	// Test QueryCount
	connCount, err := cli.QueryCount(ctx, db, "connections")
	if err != nil {
		t.Fatalf("QueryCount connections failed: %v", err)
	}
	if connCount != 2 {
		t.Errorf("expected connections count to be 2, got %d", connCount)
	}

	msgCount, err := cli.QueryCount(ctx, db, "messages")
	if err != nil {
		t.Fatalf("QueryCount messages failed: %v", err)
	}
	if msgCount != 0 {
		t.Errorf("expected messages count to be 0, got %d", msgCount)
	}

	// 3. Status Construction logic
	status := control.NewStatus("linkedinclaw", "2 connections")
	if status.AppID != "linkedinclaw" {
		t.Errorf("expected AppID 'linkedinclaw', got %q", status.AppID)
	}
	if status.Summary != "2 connections" {
		t.Errorf("expected Summary '2 connections', got %q", status.Summary)
	}
}
