package sync_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
	"github.com/pooriaarab/linkedinclaw/internal/sync"
	"github.com/pooriaarab/linkedinclaw/internal/voyager"
)

func TestRun_Integration(t *testing.T) {
	ctx := context.Background()

	// 1. Open a temporary SQLite DB via crawlkit/store
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_sync.db")

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

	// 3. Setup a mock Voyager API server
	var savedPostsRequestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/identity/profiles/me":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{ "urn": "urn:li:fs_miniProfile:12345", "firstName": "John", "lastName": "Doe", "headline": "Software Engineer" }`))
		case "/relationships/connections":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{ "elements": [ { "urn": "urn:li:fs_miniProfile:67890", "firstName": "Jane", "lastName": "Smith", "headline": "Product Manager", "company": "Google", "connectedAt": "2026-07-01T12:00:00Z" } ] }`))
		case "/messaging/conversations":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{ "elements": [ { "urn": "urn:li:fs_conversation:1", "participants": "Jane Smith", "lastActivityAt": "2026-07-02T10:00:00Z" } ] }`))
		case "/messaging/conversations/urn:li:fs_conversation:1/events":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{ "elements": [ { "urn": "urn:li:fs_message:1", "conversationUrn": "urn:li:fs_conversation:1", "senderUrn": "urn:li:fs_miniProfile:67890", "body": "Hello World!", "sentAt": "2026-07-02T10:00:00Z" } ] }`))
		case "/identity/profiles/me/posts":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{ "elements": [ { "urn": "urn:li:fs_post:1", "body": "My first claw post!", "postedAt": "2026-07-03T14:00:00Z", "likeCount": 10, "commentCount": 2 } ] }`))
		case "/identity/profiles/me/saved-posts":
			savedPostsRequestCount++
			w.WriteHeader(http.StatusTooManyRequests) // 429 to trigger deferral
		case "/relationships/followed-companies":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{ "elements": [ { "urn": "urn:li:fs_followingCompany:333", "name": "Google", "followedAt": "2026-07-09T11:00:00Z" } ] }`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// 4. Create client overriding parameters
	client := voyager.NewClient("fake-li-at", "fake-jsessionid")
	client.BaseURL = server.URL
	client.Limiter = nil
	client.BackoffBase = 1 * time.Millisecond

	// 5. Run sync
	summary, err := sync.Run(ctx, db, client, sync.SourceAPI, false)
	if err != nil {
		t.Fatalf("sync.Run returned error: %v", err)
	}

	// 6. Assert results
	// Expect saved_posts in Deferred
	var savedPostsDeferred bool
	for _, cat := range summary.Deferred {
		if cat == schema.CategorySavedPosts {
			savedPostsDeferred = true
			break
		}
	}
	if !savedPostsDeferred {
		t.Errorf("expected saved_posts to be deferred, but deferred list is: %v", summary.Deferred)
	}

	// Expect profile, connections, conversations, posts, companies_followed in Completed
	completedMap := make(map[string]bool)
	for _, cat := range summary.Completed {
		completedMap[cat] = true
	}

	expectedCompleted := []string{
		"profile",
		schema.CategoryConnections,
		schema.CategoryConversations,
		schema.CategoryPosts,
		schema.CategoryCompaniesFollowed,
	}

	for _, expected := range expectedCompleted {
		if !completedMap[expected] {
			t.Errorf("expected %q to be completed, but completed list is: %v", expected, summary.Completed)
		}
	}

	// Verify that 429 endpoint was indeed retried 5 times
	const expectedRetries = 5
	if savedPostsRequestCount != expectedRetries {
		t.Errorf("expected saved-posts endpoint to be called %d times, got %d", expectedRetries, savedPostsRequestCount)
	}

	// 7. Verify rows in database tables
	tableChecks := []struct {
		table string
		query string
		count int
	}{
		{"profile", "SELECT COUNT(*) FROM profile", 1},
		{"connections", "SELECT COUNT(*) FROM connections", 1},
		{"conversations", "SELECT COUNT(*) FROM conversations", 1},
		{"messages", "SELECT COUNT(*) FROM messages", 1},
		{"posts", "SELECT COUNT(*) FROM posts", 1},
		{"companies_followed", "SELECT COUNT(*) FROM companies_followed", 1},
		{"saved_posts", "SELECT COUNT(*) FROM saved_posts", 0},
	}

	for _, tc := range tableChecks {
		var cnt int
		if err := db.QueryRowContext(ctx, tc.query).Scan(&cnt); err != nil {
			t.Fatalf("failed to query table %s: %v", tc.table, err)
		}
		if cnt != tc.count {
			t.Errorf("expected %d rows in table %s, got %d", tc.count, tc.table, cnt)
		}
	}

	// 8. Verify FTS indexing of messages
	var ftsCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'World'").Scan(&ftsCount)
	if err != nil {
		t.Fatalf("failed to query messages_fts: %v", err)
	}
	if ftsCount != 1 {
		t.Errorf("expected 1 row matching 'World' in messages_fts, got %d", ftsCount)
	}
}

func TestRun_SourceExport(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_sync_export.db")

	s, err := store.Open(ctx, store.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("failed to open crawlkit store: %v", err)
	}
	defer s.Close()

	db := s.DB()
	_ = schema.Migrate(db)

	client := voyager.NewClient("fake-li-at", "fake-jsessionid")
	summary, err := sync.Run(ctx, db, client, sync.SourceExport, false)
	if err != nil {
		t.Fatalf("sync.Run for SourceExport returned error: %v", err)
	}

	if len(summary.Deferred) != 6 {
		t.Errorf("expected 6 deferred categories for SourceExport, got %d: %v", len(summary.Deferred), summary.Deferred)
	}
	if summary.Comment == "" {
		t.Error("expected comment explaining export-only sync in SourceExport path")
	}
}

func TestRun_SourceBoth_Downloads(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_sync_both.db")

	s, err := store.Open(ctx, store.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("failed to open crawlkit store: %v", err)
	}
	defer s.Close()

	db := s.DB()
	_ = schema.Migrate(db)

	// Set DownloadsDir to a temp dir where we'll write a fake zip
	origDownloadsDir := sync.DownloadsDir
	defer func() { sync.DownloadsDir = origDownloadsDir }()

	sync.DownloadsDir = t.TempDir()

	// 1. Setup mock Voyager API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/identity/profiles/me":
			_, _ = w.Write([]byte(`{ "urn": "urn:li:fs_miniProfile:12345", "firstName": "John", "lastName": "Doe" }`))
		case "/relationships/connections":
			_, _ = w.Write([]byte(`{ "elements": [] }`))
		case "/messaging/conversations":
			_, _ = w.Write([]byte(`{ "elements": [] }`))
		case "/identity/profiles/me/posts":
			_, _ = w.Write([]byte(`{ "elements": [] }`))
		case "/identity/profiles/me/saved-posts":
			_, _ = w.Write([]byte(`{ "elements": [] }`))
		case "/relationships/followed-companies":
			_, _ = w.Write([]byte(`{ "elements": [] }`))
		}
	}))
	defer server.Close()

	client := voyager.NewClient("fake-li-at", "fake-jsessionid")
	client.BaseURL = server.URL
	client.Limiter = nil

	// No zips exist initially
	summary, err := sync.Run(ctx, db, client, sync.SourceBoth, false)
	if err != nil {
		t.Fatalf("sync.Run for SourceBoth returned error: %v", err)
	}
	if len(summary.FoundExportZips) != 0 {
		t.Errorf("expected no export zips found, got %d", len(summary.FoundExportZips))
	}

	// Create a dummy zip file
	dummyZipPath := filepath.Join(sync.DownloadsDir, "export.zip")
	if err := os.WriteFile(dummyZipPath, []byte("fake-zip-data"), 0644); err != nil {
		t.Fatalf("failed to create fake zip file: %v", err)
	}

	// Now zips should be found
	summaryWithZips, err := sync.Run(ctx, db, client, sync.SourceBoth, false)
	if err != nil {
		t.Fatalf("sync.Run for SourceBoth returned error: %v", err)
	}
	if len(summaryWithZips.FoundExportZips) != 1 {
		t.Errorf("expected 1 export zip found, got %d: %v", len(summaryWithZips.FoundExportZips), summaryWithZips.FoundExportZips)
	}
	if filepath.Base(summaryWithZips.FoundExportZips[0]) != "export.zip" {
		t.Errorf("expected export zip filename to be export.zip, got %q", summaryWithZips.FoundExportZips[0])
	}
}
