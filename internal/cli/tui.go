// This file implements the interactive TUI command which is tested manually rather than in automated tests.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/openclaw/crawlkit/config"
	"github.com/openclaw/crawlkit/store"
	"github.com/openclaw/crawlkit/tui"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
)

// TuiCmd represents the terminal user interface command.
type TuiCmd struct {
	JSON bool `name:"json" help:"Format output as JSON."`
}

// Run executes the tui command.
func (c *TuiCmd) Run() error {
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

	// 4. Load all rows and map to tui.Row
	rows, err := LoadRows(db)
	if err != nil {
		return fmt.Errorf("failed to load rows: %w", err)
	}

	// 5. Call tui.Browse
	opts := tui.BrowseOptions{
		AppName:      "linkedinclaw",
		Title:        "LinkedIn Claw Explorer",
		EmptyMessage: "No data loaded yet. Run 'linkedinclaw sync' first.",
		Rows:         rows,
		JSON:         c.JSON,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
	}

	if err := tui.Browse(ctx, opts); err != nil {
		return fmt.Errorf("tui browse failed: %w", err)
	}

	return nil
}

// LoadRows loads all rows from connections, conversations, messages, posts, saved_posts, and companies_followed
// and maps them to tui.Row.
func LoadRows(db *sql.DB) ([]tui.Row, error) {
	ctx := context.Background()
	var rows []tui.Row

	// 1. Connections
	connRows, err := db.QueryContext(ctx, `
		SELECT
			COALESCE(urn, ''),
			COALESCE(first_name, ''),
			COALESCE(last_name, ''),
			COALESCE(headline, ''),
			COALESCE(company, ''),
			COALESCE(connected_at, ''),
			COALESCE(source, '')
		FROM connections
	`)
	if err != nil {
		return nil, fmt.Errorf("query connections: %w", err)
	}
	defer connRows.Close()

	for connRows.Next() {
		var urn, firstName, lastName, headline, company, connectedAt, source string
		if err := connRows.Scan(&urn, &firstName, &lastName, &headline, &company, &connectedAt, &source); err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}
		rows = append(rows, tui.Row{
			Source:    source,
			Kind:      "connection",
			ID:        urn,
			Title:     firstName + " " + lastName,
			Text:      headline,
			Detail:    company,
			CreatedAt: connectedAt,
			UpdatedAt: connectedAt,
		})
	}
	if err := connRows.Err(); err != nil {
		return nil, fmt.Errorf("rows connections: %w", err)
	}

	// 2. Conversations
	convRows, err := db.QueryContext(ctx, `
		SELECT
			COALESCE(urn, ''),
			COALESCE(participants, ''),
			COALESCE(last_activity_at, '')
		FROM conversations
	`)
	if err != nil {
		return nil, fmt.Errorf("query conversations: %w", err)
	}
	defer convRows.Close()

	for convRows.Next() {
		var urn, participants, lastActivityAt string
		if err := convRows.Scan(&urn, &participants, &lastActivityAt); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		rows = append(rows, tui.Row{
			Kind:      "conversation",
			ID:        urn,
			Title:     participants,
			CreatedAt: lastActivityAt,
			UpdatedAt: lastActivityAt,
		})
	}
	if err := convRows.Err(); err != nil {
		return nil, fmt.Errorf("rows conversations: %w", err)
	}

	// 3. Messages
	msgRows, err := db.QueryContext(ctx, `
		SELECT
			COALESCE(m.urn, ''),
			COALESCE(m.conversation_urn, ''),
			COALESCE(m.sender_urn, ''),
			COALESCE(conn.first_name || ' ' || conn.last_name, prof.first_name || ' ' || prof.last_name, m.sender_urn) AS sender_name,
			COALESCE(m.body, ''),
			COALESCE(m.sent_at, ''),
			COALESCE(m.source, ''),
			COALESCE(conv.participants, '') AS conversation_name
		FROM messages m
		LEFT JOIN conversations conv ON m.conversation_urn = conv.urn
		LEFT JOIN connections conn ON m.sender_urn = conn.urn
		LEFT JOIN profile prof ON m.sender_urn = prof.urn
	`)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer msgRows.Close()

	for msgRows.Next() {
		var urn, conversationUrn, senderUrn, senderName, body, sentAt, source, conversationName string
		if err := msgRows.Scan(&urn, &conversationUrn, &senderUrn, &senderName, &body, &sentAt, &source, &conversationName); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		rows = append(rows, tui.Row{
			Source:    source,
			Kind:      "message",
			ID:        urn,
			ParentID:  conversationUrn,
			Container: conversationName,
			Author:    senderName,
			Title:     "Message from " + senderName,
			Text:      body,
			CreatedAt: sentAt,
			UpdatedAt: sentAt,
		})
	}
	if err := msgRows.Err(); err != nil {
		return nil, fmt.Errorf("rows messages: %w", err)
	}

	// 4. Posts
	postRows, err := db.QueryContext(ctx, `
		SELECT
			COALESCE(urn, ''),
			COALESCE(body, ''),
			COALESCE(posted_at, ''),
			COALESCE(like_count, 0),
			COALESCE(comment_count, 0)
		FROM posts
	`)
	if err != nil {
		return nil, fmt.Errorf("query posts: %w", err)
	}
	defer postRows.Close()

	for postRows.Next() {
		var urn, body, postedAt string
		var likeCount, commentCount int
		if err := postRows.Scan(&urn, &body, &postedAt, &likeCount, &commentCount); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		title := body
		if len(title) > 80 {
			title = title[:80] + "..."
		}
		rows = append(rows, tui.Row{
			Kind:      "post",
			ID:        urn,
			Title:     title,
			Text:      body,
			Detail:    fmt.Sprintf("Likes: %d, Comments: %d", likeCount, commentCount),
			CreatedAt: postedAt,
			UpdatedAt: postedAt,
		})
	}
	if err := postRows.Err(); err != nil {
		return nil, fmt.Errorf("rows posts: %w", err)
	}

	// 5. Saved Posts
	savedRows, err := db.QueryContext(ctx, `
		SELECT
			COALESCE(urn, ''),
			COALESCE(author, ''),
			COALESCE(body, ''),
			COALESCE(saved_at, '')
		FROM saved_posts
	`)
	if err != nil {
		return nil, fmt.Errorf("query saved_posts: %w", err)
	}
	defer savedRows.Close()

	for savedRows.Next() {
		var urn, author, body, savedAt string
		if err := savedRows.Scan(&urn, &author, &body, &savedAt); err != nil {
			return nil, fmt.Errorf("scan saved post: %w", err)
		}
		rows = append(rows, tui.Row{
			Kind:      "saved_post",
			ID:        urn,
			Author:    author,
			Title:     "Saved post by " + author,
			Text:      body,
			CreatedAt: savedAt,
			UpdatedAt: savedAt,
		})
	}
	if err := savedRows.Err(); err != nil {
		return nil, fmt.Errorf("rows saved_posts: %w", err)
	}

	// 6. Companies Followed
	compRows, err := db.QueryContext(ctx, `
		SELECT
			COALESCE(urn, ''),
			COALESCE(name, ''),
			COALESCE(followed_at, '')
		FROM companies_followed
	`)
	if err != nil {
		return nil, fmt.Errorf("query companies_followed: %w", err)
	}
	defer compRows.Close()

	for compRows.Next() {
		var urn, name, followedAt string
		if err := compRows.Scan(&urn, &name, &followedAt); err != nil {
			return nil, fmt.Errorf("scan company: %w", err)
		}
		rows = append(rows, tui.Row{
			Kind:      "company",
			ID:        urn,
			Title:     name,
			CreatedAt: followedAt,
			UpdatedAt: followedAt,
		})
	}
	if err := compRows.Err(); err != nil {
		return nil, fmt.Errorf("rows companies_followed: %w", err)
	}

	return rows, nil
}
