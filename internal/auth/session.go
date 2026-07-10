package auth

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// storeFunc is a package-level variable pointing to Store,
// allowing tests to mock the keyring storage behavior.
var storeFunc = Store

// Resolve returns the linkedin session credentials.
// It first checks the environment variables LINKEDINCLAW_LI_AT and LINKEDINCLAW_JSESSIONID.
// If both are set, it returns them directly.
// Otherwise, it falls back to the macOS OS keyring (via security CLI).
func Resolve() (liAt, jsessionID string, err error) {
	liAtEnv := os.Getenv("LINKEDINCLAW_LI_AT")
	jsessionIDEnv := os.Getenv("LINKEDINCLAW_JSESSIONID")
	if liAtEnv != "" && jsessionIDEnv != "" {
		return liAtEnv, jsessionIDEnv, nil
	}

	if runtime.GOOS != "darwin" {
		return "", "", fmt.Errorf("keyring access is only supported on macOS (darwin)")
	}

	liAt, err = findPassword("li_at")
	if err != nil {
		return "", "", fmt.Errorf("resolve li_at: %w", err)
	}

	jsessionID, err = findPassword("jsessionid")
	if err != nil {
		return "", "", fmt.Errorf("resolve jsessionid: %w", err)
	}

	return liAt, jsessionID, nil
}

// Store saves the credentials to the macOS OS keyring (via security CLI).
func Store(liAt, jsessionID string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("keyring access is only supported on macOS (darwin)")
	}

	if err := addPassword("li_at", liAt); err != nil {
		return fmt.Errorf("store li_at: %w", err)
	}

	if err := addPassword("jsessionid", jsessionID); err != nil {
		return fmt.Errorf("store jsessionid: %w", err)
	}

	return nil
}

func findPassword(account string) (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-s", "linkedinclaw", "-a", account, "-w")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("exec security find-generic-password failed: %v, stderr: %q", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func addPassword(account, password string) error {
	cmd := exec.Command("security", "add-generic-password", "-U", "-s", "linkedinclaw", "-a", account, "-w", password)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec security add-generic-password failed: %v, stderr: %q", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
