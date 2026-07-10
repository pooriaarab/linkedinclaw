package cli

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/openclaw/crawlkit/config"
	"github.com/pooriaarab/linkedinclaw/internal/auth"
)

// ServeSessionCmd represents the serve-session subcommand.
type ServeSessionCmd struct {
	Port int `help:"Port to listen on for cookie updates." default:"9090"`
}

// Run executes the serve-session command.
func (c *ServeSessionCmd) Run() error {
	app := config.App{Name: "linkedinclaw"}
	paths, err := app.DefaultPaths()
	if err != nil {
		return fmt.Errorf("failed to get default paths: %w", err)
	}

	sessionFilePath := filepath.Join(filepath.Dir(paths.ConfigPath), "session.json")

	fmt.Printf("Starting session server on 127.0.0.1:%d...\n", c.Port)
	fmt.Printf("Session updates will be written to: %s\n", sessionFilePath)

	server, err := auth.Listen(c.Port, sessionFilePath)
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	// Block until interrupted
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down session server...")
	_ = server.Close()
	return nil
}
