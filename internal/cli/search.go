package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/openclaw/crawlkit/config"
	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
)

// Result is a small search result hit.
type Result struct {
	Kind    string `json:"kind"`
	Urn     string `json:"urn"`
	Snippet string `json:"snippet"`
}

// SearchCmd represents the full-text search command.
type SearchCmd struct {
	Query string `arg:"" help:"Search query term."`
	JSON  bool   `help:"Format output as JSON."`
}

// Run executes the search command.
func (c *SearchCmd) Run() error {
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

	// 4. Query results
	results, err := Query(db, c.Query)
	if err != nil {
		return fmt.Errorf("search query failed: %w", err)
	}

	// 5. Output
	if c.JSON {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	for _, r := range results {
		fmt.Printf("[%s] %s: %s\n", r.Kind, r.Urn, r.Snippet)
	}

	return nil
}

// Query runs three separate FTS5 MATCH queries against messages_fts, posts_fts, and saved_posts_fts,
// unions the results in application code, and returns them concatenated in that order.
func Query(db *sql.DB, term string) ([]Result, error) {
	ctx := context.Background()
	var results []Result

	// 1. Messages
	rowsMsg, err := db.QueryContext(ctx, `
		SELECT m.urn, snippet(messages_fts, 0, '[', ']', '...', 10)
		FROM messages m
		JOIN messages_fts ON messages_fts.rowid = m.rowid
		WHERE messages_fts MATCH ?
		ORDER BY rank
	`, term)
	if err == nil {
		defer rowsMsg.Close()
		for rowsMsg.Next() {
			var urn, snippet string
			if err := rowsMsg.Scan(&urn, &snippet); err != nil {
				return nil, fmt.Errorf("scan message result: %w", err)
			}
			results = append(results, Result{
				Kind:    "message",
				Urn:     urn,
				Snippet: snippet,
			})
		}
		if err := rowsMsg.Err(); err != nil {
			return nil, fmt.Errorf("rows message error: %w", err)
		}
	} else {
		return nil, fmt.Errorf("messages fts query: %w", err)
	}

	// 2. Posts
	rowsPost, err := db.QueryContext(ctx, `
		SELECT p.urn, snippet(posts_fts, 0, '[', ']', '...', 10)
		FROM posts p
		JOIN posts_fts ON posts_fts.rowid = p.rowid
		WHERE posts_fts MATCH ?
		ORDER BY rank
	`, term)
	if err == nil {
		defer rowsPost.Close()
		for rowsPost.Next() {
			var urn, snippet string
			if err := rowsPost.Scan(&urn, &snippet); err != nil {
				return nil, fmt.Errorf("scan post result: %w", err)
			}
			results = append(results, Result{
				Kind:    "post",
				Urn:     urn,
				Snippet: snippet,
			})
		}
		if err := rowsPost.Err(); err != nil {
			return nil, fmt.Errorf("rows post error: %w", err)
		}
	} else {
		return nil, fmt.Errorf("posts fts query: %w", err)
	}

	// 3. Saved Posts
	rowsSaved, err := db.QueryContext(ctx, `
		SELECT s.urn, snippet(saved_posts_fts, 0, '[', ']', '...', 10)
		FROM saved_posts s
		JOIN saved_posts_fts ON saved_posts_fts.rowid = s.rowid
		WHERE saved_posts_fts MATCH ?
		ORDER BY rank
	`, term)
	if err == nil {
		defer rowsSaved.Close()
		for rowsSaved.Next() {
			var urn, snippet string
			if err := rowsSaved.Scan(&urn, &snippet); err != nil {
				return nil, fmt.Errorf("scan saved post result: %w", err)
			}
			results = append(results, Result{
				Kind:    "saved_post",
				Urn:     urn,
				Snippet: snippet,
			})
		}
		if err := rowsSaved.Err(); err != nil {
			return nil, fmt.Errorf("rows saved post error: %w", err)
		}
	} else {
		return nil, fmt.Errorf("saved_posts fts query: %w", err)
	}

	return results, nil
}
