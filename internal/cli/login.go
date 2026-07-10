// This file drives a real browser interactively, not realistically unit-testable.
package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pooriaarab/linkedinclaw/internal/auth"
)

// LoginCmd represents the login subcommand.
type LoginCmd struct{}

// Run executes the login command.
func (c *LoginCmd) Run() (err error) {
	// Defer closing the agent-browser session
	defer func() {
		if closeErr := runCmd("agent-browser", "close"); closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				fmt.Fprintf(os.Stderr, "Warning: failed to close agent-browser: %v\n", closeErr)
			}
		}
	}()

	// 1. Open the page
	fmt.Println("Opening LinkedIn in Chrome...")
	if err := runCmd("agent-browser", "--profile", "Default", "open", "https://www.linkedin.com/feed/"); err != nil {
		return fmt.Errorf("failed to open LinkedIn feed: %w", err)
	}

	// 2. Wait for page to settle
	if err := runCmd("agent-browser", "wait", "3000"); err != nil {
		return fmt.Errorf("failed during agent-browser wait: %w", err)
	}

	// Prompt the user to log in/complete 2FA manually
	fmt.Println("Complete login in the browser window if prompted, then press Enter here to continue.")
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}

	// 3. Extract cookies
	fmt.Println("Extracting cookies...")
	cookiesBytes, err := runCmdWithOutput("agent-browser", "cookies", "get", "--json")
	if err != nil {
		return fmt.Errorf("failed to get cookies from browser: %w", err)
	}

	type Cookie struct {
		Domain   string  `json:"domain"`
		Expires  float64 `json:"expires"`
		HTTPOnly bool    `json:"httpOnly"`
		Name     string  `json:"name"`
		Path     string  `json:"path"`
		Secure   bool    `json:"secure"`
		Session  bool    `json:"session"`
		Size     int     `json:"size"`
		Value    string  `json:"value"`
	}

	var wrapper struct {
		Success bool `json:"success"`
		Data    struct {
			Cookies []Cookie `json:"cookies"`
		} `json:"data"`
		Error interface{} `json:"error"`
	}

	if err := json.Unmarshal(cookiesBytes, &wrapper); err != nil {
		return fmt.Errorf("failed to parse cookie JSON output: %w", err)
	}

	cookies := wrapper.Data.Cookies

	var liAt, jsessionID string
	for _, cookie := range cookies {
		if strings.Contains(strings.ToLower(cookie.Domain), "linkedin.com") {
			if cookie.Name == "li_at" {
				liAt = cookie.Value
			} else if cookie.Name == "JSESSIONID" {
				jsessionID = cookie.Value
			}
		}
	}

	// 4. Validate cookies
	if liAt == "" || jsessionID == "" {
		fmt.Println("login not detected -- run `linkedinclaw login` again once you've finished signing in.")
		return fmt.Errorf("missing required LinkedIn cookies (li_at, JSESSIONID)")
	}

	// 5. Store session and print confirmation
	if err := auth.Store(liAt, jsessionID); err != nil {
		return fmt.Errorf("failed to store session in keyring: %w", err)
	}

	liAtShort := liAt
	if len(liAtShort) > 6 {
		liAtShort = liAtShort[:6]
	}
	fmt.Printf("Session stored (li_at: %s...)\n", liAtShort)

	return nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %q %v failed: %w (stderr: %q, stdout: %q)", name, args, err, strings.TrimSpace(stderr.String()), strings.TrimSpace(stdout.String()))
	}
	return nil
}

func runCmdWithOutput(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command %q %v failed: %w (stderr: %q, stdout: %q)", name, args, err, strings.TrimSpace(stderr.String()), strings.TrimSpace(stdout.String()))
	}
	return stdout.Bytes(), nil
}
