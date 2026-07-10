package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/openclaw/crawlkit/config"
	"github.com/openclaw/crawlkit/store"
	"github.com/pooriaarab/linkedinclaw/internal/auth"
	"github.com/pooriaarab/linkedinclaw/internal/schema"
	"github.com/pooriaarab/linkedinclaw/internal/voyager"
)

// DoctorCmd represents the doctor command.
type DoctorCmd struct{}

// Run executes the doctor checks.
func (c *DoctorCmd) Run() error {
	var passed, total int

	// 1. Config loads
	total++
	app := config.App{Name: "linkedinclaw"}
	paths, err := app.DefaultPaths()
	if err != nil {
		fmt.Printf("✗ Config load failed: %v\n", err)
		fmt.Printf("%d/%d checks passed.\n", passed, total)
		return fmt.Errorf("config load failed: %w", err)
	}
	passed++
	fmt.Printf("✓ Config paths loaded (db: %s)\n", paths.DBPath)

	// 2. Token resolution
	total++
	liAt, jsessionID, err := auth.Resolve()
	hasCredentials := err == nil
	if hasCredentials {
		source := "keyring"
		if os.Getenv("LINKEDINCLAW_LI_AT") != "" && os.Getenv("LINKEDINCLAW_JSESSIONID") != "" {
			source = "environment variables"
		}
		passed++
		fmt.Printf("✓ Credentials found (source: %s)\n", source)
	} else {
		fmt.Println("✗ Credentials missing: run `linkedinclaw login` to authenticate")
	}

	// 3. Voyager connection
	total++
	if hasCredentials {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, voyagerErr := voyager.NewClient(liAt, jsessionID).FetchProfile(ctx)
		cancel()
		if voyagerErr == nil {
			passed++
			fmt.Println("✓ Voyager connection: authentication verified")
		} else {
			fmt.Printf("✗ Voyager connection failed: %v\n", voyagerErr)
		}
	} else {
		fmt.Println("✗ Voyager connection failed: credentials missing")
	}

	// 4. DB + FTS wiring
	total++
	dbWiringErr := func() error {
		ctx := context.Background()
		s, err := store.Open(ctx, store.Options{
			Path: paths.DBPath,
		})
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer s.Close()

		db := s.DB()
		if err := schema.Migrate(db); err != nil {
			return fmt.Errorf("schema migrate: %w", err)
		}

		var ftsCount int
		err = db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('messages_fts', 'posts_fts', 'saved_posts_fts')").Scan(&ftsCount)
		if err != nil {
			return fmt.Errorf("query sqlite_master: %w", err)
		}
		if ftsCount != 3 {
			return fmt.Errorf("expected 3 FTS tables, found %d", ftsCount)
		}
		return nil
	}()

	if dbWiringErr == nil {
		passed++
		fmt.Println("✓ Database and FTS5 wiring successfully verified")
	} else {
		fmt.Printf("✗ Database and FTS5 wiring failed: %v\n", dbWiringErr)
		fmt.Printf("%d/%d checks passed.\n", passed, total)
		return fmt.Errorf("database wiring failed: %w", dbWiringErr)
	}

	fmt.Printf("%d/%d checks passed.\n", passed, total)
	return nil
}
