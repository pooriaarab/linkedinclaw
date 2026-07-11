package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openclaw/crawlkit/state"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
	"github.com/pooriaarab/linkedinclaw/internal/voyager"
)

// Source represents the synchronization data source.
type Source int

const (
	SourceAPI Source = iota
	SourceExport
	SourceBoth
)

// DownloadsDir is the path to search for GDPR zip export archives when using SourceBoth.
var DownloadsDir = "~/Downloads"

// Summary describes the outcome of a sync run.
type Summary struct {
	Completed       []string `json:"completed"`
	Deferred        []string `json:"deferred"`
	Comment         string   `json:"comment,omitempty"`
	FoundExportZips []string `json:"found_export_zips,omitempty"`
}

func resolveDownloadsDir() (string, error) {
	if strings.HasPrefix(DownloadsDir, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, DownloadsDir[1:]), nil
	}
	return DownloadsDir, nil
}

// Run executes the LinkedIn data sync flow.
func Run(ctx context.Context, db *sql.DB, client *voyager.Client, source Source, full bool) (Summary, error) {
	var summary Summary

	if source == SourceExport {
		summary.Comment = "export-only sync performs no API scan -- use `linkedinclaw export import <zip>` directly"
		return summary, nil
	}

	scopedStore := state.NewScoped(db)

	categories := []struct {
		name string
		sync func(ctx context.Context, tx *sql.Tx) error
	}{
		{
			name: "profile",
			sync: func(ctx context.Context, tx *sql.Tx) error {
				p, err := client.FetchProfile(ctx)
				if err != nil {
					return err
				}
				_, err = tx.ExecContext(ctx, `
					INSERT OR REPLACE INTO profile (id, urn, first_name, last_name, headline, updated_at)
					VALUES (1, ?, ?, ?, ?, ?);
				`, p.MiniProfile.EntityUrn, p.MiniProfile.FirstName, p.MiniProfile.LastName, p.MiniProfile.Occupation, time.Now().Format(time.RFC3339))
				return err
			},
		},
		{
			name: schema.CategoryConnections,
			sync: func(ctx context.Context, tx *sql.Tx) error {
				conns, err := client.FetchConnections(ctx)
				if err != nil {
					return err
				}
				for _, conn := range conns {
					_, err = tx.ExecContext(ctx, `
						INSERT OR REPLACE INTO connections (urn, first_name, last_name, headline, company, connected_at, source)
						VALUES (?, ?, ?, ?, ?, ?, 'api');
					`, conn.Urn, conn.FirstName, conn.LastName, conn.Headline, conn.Company, conn.ConnectedAt)
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			name: schema.CategoryConversations,
			sync: func(ctx context.Context, tx *sql.Tx) error {
				convs, err := client.FetchConversations(ctx)
				if err != nil {
					return err
				}
				for _, conv := range convs {
					_, err = tx.ExecContext(ctx, `
						INSERT OR REPLACE INTO conversations (urn, participants, last_activity_at)
						VALUES (?, ?, ?);
					`, conv.Urn, conv.Participants, conv.LastActivityAt)
					if err != nil {
						return err
					}

					msgs, err := client.FetchMessages(ctx, conv.Urn)
					if err != nil {
						return err
					}
					for _, msg := range msgs {
						_, err = tx.ExecContext(ctx, `
							INSERT OR REPLACE INTO messages (urn, conversation_urn, sender_urn, body, sent_at, source)
							VALUES (?, ?, ?, ?, ?, 'api');
						`, msg.Urn, msg.ConversationUrn, msg.SenderUrn, msg.Body, msg.SentAt)
						if err != nil {
							return err
						}
					}
				}
				return nil
			},
		},
		{
			name: schema.CategoryPosts,
			sync: func(ctx context.Context, tx *sql.Tx) error {
				posts, err := client.FetchOwnPosts(ctx)
				if err != nil {
					return err
				}
				for _, p := range posts {
					_, err = tx.ExecContext(ctx, `
						INSERT OR REPLACE INTO posts (urn, body, posted_at, like_count, comment_count)
						VALUES (?, ?, ?, ?, ?);
					`, p.Urn, p.Body, p.PostedAt, p.LikeCount, p.CommentCount)
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			name: schema.CategorySavedPosts,
			sync: func(ctx context.Context, tx *sql.Tx) error {
				saved, err := client.FetchSavedPosts(ctx)
				if err != nil {
					return err
				}
				for _, p := range saved {
					_, err = tx.ExecContext(ctx, `
						INSERT OR REPLACE INTO saved_posts (urn, author, body, saved_at)
						VALUES (?, ?, ?, ?);
					`, p.Urn, p.Author, p.Body, p.SavedAt)
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			name: schema.CategoryCompaniesFollowed,
			sync: func(ctx context.Context, tx *sql.Tx) error {
				companies, err := client.FetchFollowedCompanies(ctx)
				if err != nil {
					return err
				}
				for _, c := range companies {
					_, err = tx.ExecContext(ctx, `
						INSERT OR REPLACE INTO companies_followed (urn, name, followed_at)
						VALUES (?, ?, ?);
					`, c.Urn, c.Name, c.FollowedAt)
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
	}

	for _, cat := range categories {
		if full {
			if err := scopedStore.Set(ctx, cat.name, ""); err != nil {
				return summary, fmt.Errorf("failed to reset cursor for %s: %w", cat.name, err)
			}
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return summary, fmt.Errorf("failed to begin transaction: %w", err)
		}

		if err := cat.sync(ctx, tx); err != nil {
			_ = tx.Rollback()
			if errors.Is(err, voyager.ErrDeferred) {
				summary.Deferred = append(summary.Deferred, cat.name)
				continue
			}
			return summary, fmt.Errorf("failed to sync category %s: %w", cat.name, err)
		}

		if err := tx.Commit(); err != nil {
			return summary, fmt.Errorf("failed to commit transaction for %s: %w", cat.name, err)
		}

		if err := scopedStore.Set(ctx, cat.name, time.Now().Format(time.RFC3339)); err != nil {
			return summary, fmt.Errorf("failed to save cursor for %s: %w", cat.name, err)
		}

		summary.Completed = append(summary.Completed, cat.name)
	}

	// Rebuild FTS tables to ensure full-text search indexes are up-to-date
	_, _ = db.ExecContext(ctx, "INSERT INTO messages_fts(messages_fts) VALUES('rebuild');")
	_, _ = db.ExecContext(ctx, "INSERT INTO posts_fts(posts_fts) VALUES('rebuild');")
	_, _ = db.ExecContext(ctx, "INSERT INTO saved_posts_fts(saved_posts_fts) VALUES('rebuild');")

	if source == SourceBoth {
		resolved, err := resolveDownloadsDir()
		if err == nil {
			files, err := filepath.Glob(filepath.Join(resolved, "*.zip"))
			if err == nil && len(files) > 0 {
				summary.FoundExportZips = files
			}
		}
	}

	return summary, nil
}
