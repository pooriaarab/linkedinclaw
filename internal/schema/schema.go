package schema

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/openclaw/crawlkit/state"
)

// Tracked categories for sync state tracking.
const (
	CategoryConnections       = "connections"
	CategoryConversations     = "conversations"
	CategoryPosts             = "posts"
	CategorySavedPosts        = "saved_posts"
	CategoryCompaniesFollowed = "companies_followed"
)

// Migrate runs the SQLite schema migrations for linkedinclaw,
// including creating tables, virtual tables for FTS5, and the sync state schema.
func Migrate(db *sql.DB) error {
	ctx := context.Background()

	// 1. Ensure the crawlkit/state schema is applied
	if err := state.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("ensure state schema: %w", err)
	}
	// sync.Run uses the scoped cursor variant (state.NewScoped), which needs its own table.
	if err := state.EnsureScopedSchema(ctx, db); err != nil {
		return fmt.Errorf("ensure scoped state schema: %w", err)
	}

	// 2. Execute the LinkedInclaw-specific table definitions
	queries := []string{
		`CREATE TABLE IF NOT EXISTS profile (id INTEGER PRIMARY KEY CHECK (id=1), urn TEXT, first_name TEXT, last_name TEXT, headline TEXT, updated_at TEXT);`,
		`CREATE TABLE IF NOT EXISTS connections (urn TEXT PRIMARY KEY, first_name TEXT, last_name TEXT, headline TEXT, company TEXT, connected_at TEXT, source TEXT);`,
		`CREATE TABLE IF NOT EXISTS conversations (urn TEXT PRIMARY KEY, participants TEXT, last_activity_at TEXT);`,
		`CREATE TABLE IF NOT EXISTS messages (urn TEXT PRIMARY KEY, conversation_urn TEXT REFERENCES conversations(urn), sender_urn TEXT, body TEXT, sent_at TEXT, source TEXT);`,
		`CREATE TABLE IF NOT EXISTS posts (urn TEXT PRIMARY KEY, body TEXT, posted_at TEXT, like_count INTEGER, comment_count INTEGER);`,
		`CREATE TABLE IF NOT EXISTS saved_posts (urn TEXT PRIMARY KEY, author TEXT, body TEXT, saved_at TEXT);`,
		`CREATE TABLE IF NOT EXISTS companies_followed (urn TEXT PRIMARY KEY, name TEXT, followed_at TEXT);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(body, content='messages', content_rowid='rowid');`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(body, content='posts', content_rowid='rowid');`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS saved_posts_fts USING fts5(body, content='saved_posts', content_rowid='rowid');`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("execute schema query: %w", err)
		}
	}

	return nil
}
