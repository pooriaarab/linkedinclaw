package cli

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/openclaw/crawlkit/config"
	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
)

// MessageQueryResult represents a matching message record.
type MessageQueryResult struct {
	Sender string
	Body   string
	SentAt string
}

// MessagesCmd represents the messages query command.
type MessagesCmd struct {
	Person string `name:"person" help:"Filter messages by participant's first_name or last_name (using LIKE)." default:""`
	Hours  int    `name:"hours" help:"Filter messages sent within the last N hours (0/omitted means no time limit)." default:"0"`
}

// Run executes the messages subcommand.
func (c *MessagesCmd) Run() error {
	ctx := context.Background()

	// 1. Resolve database path using crawlkit config
	app := config.App{
		Name: "linkedinclaw",
	}
	paths, err := app.DefaultPaths()
	if err != nil {
		return fmt.Errorf("failed to resolve application paths: %w", err)
	}

	// 2. Open SQLite database using crawlkit store
	s, err := store.Open(ctx, store.Options{
		Path: paths.DBPath,
	})
	if err != nil {
		return fmt.Errorf("failed to open database at %s: %w", paths.DBPath, err)
	}
	defer s.Close()

	db := s.DB()

	// 3. Ensure migrations are applied
	if err := schema.Migrate(db); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	// 4. Query messages
	results, err := QueryMessages(db, c.Person, c.Hours)
	if err != nil {
		return fmt.Errorf("query messages failed: %w", err)
	}

	// 5. Output
	for _, r := range results {
		fmt.Printf("%s (%s): %s\n", r.Sender, r.SentAt, r.Body)
	}

	return nil
}

// QueryMessages filters messages joined to conversations and connections.
func QueryMessages(db *sql.DB, person string, hours int) ([]MessageQueryResult, error) {
	ctx := context.Background()

	query := `
		SELECT
			COALESCE(conn.first_name || ' ' || conn.last_name, m.sender_urn) AS sender,
			COALESCE(m.body, '') AS body,
			COALESCE(m.sent_at, '') AS sent_at
		FROM messages m
		JOIN conversations conv ON m.conversation_urn = conv.urn
		JOIN connections conn ON m.sender_urn = conn.urn
		WHERE 1=1
	`
	var args []interface{}

	if person != "" {
		query += ` AND (conn.first_name LIKE ? OR conn.last_name LIKE ? OR (conn.first_name || ' ' || conn.last_name) LIKE ?)`
		pattern := "%" + person + "%"
		args = append(args, pattern, pattern, pattern)
	}

	if hours > 0 {
		threshold := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
		thresholdStr := threshold.Format("2006-01-02 15:04:05")
		query += ` AND datetime(m.sent_at) >= datetime(?)`
		args = append(args, thresholdStr)
	}

	query += ` ORDER BY datetime(m.sent_at) ASC`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query execute failed: %w", err)
	}
	defer rows.Close()

	var results []MessageQueryResult
	for rows.Next() {
		var r MessageQueryResult
		if err := rows.Scan(&r.Sender, &r.Body, &r.SentAt); err != nil {
			return nil, fmt.Errorf("scan message row: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return results, nil
}
