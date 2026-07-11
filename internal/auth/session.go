package auth

import (
	"fmt"
	"os"
	"runtime"

	"github.com/zalando/go-keyring"
)

// storeFunc is a package-level variable pointing to Store,
// allowing tests to mock the keyring storage behavior.
var storeFunc = Store

// keyringService is the generic-password service name used in the Keychain.
// Verified compatible with standard macOS Keychain Generic Password API.
var keyringService = "linkedinclaw"

// Resolve returns the linkedin session credentials.
// It first checks the environment variables LINKEDINCLAW_LI_AT and LINKEDINCLAW_JSESSIONID.
// If both are set, it returns them directly.
// Otherwise, it falls back to the macOS OS keyring (via go-keyring).
func Resolve() (liAt, jsessionID string, err error) {
	liAtEnv := os.Getenv("LINKEDINCLAW_LI_AT")
	jsessionIDEnv := os.Getenv("LINKEDINCLAW_JSESSIONID")
	if liAtEnv != "" && jsessionIDEnv != "" {
		return liAtEnv, jsessionIDEnv, nil
	}

	if runtime.GOOS != "darwin" {
		return "", "", fmt.Errorf("keyring access is only supported on macOS (darwin)")
	}

	liAt, err = keyring.Get(keyringService, "li_at")
	if err != nil {
		return "", "", fmt.Errorf("resolve li_at: %w", err)
	}

	jsessionID, err = keyring.Get(keyringService, "jsessionid")
	if err != nil {
		return "", "", fmt.Errorf("resolve jsessionid: %w", err)
	}

	return liAt, jsessionID, nil
}

// Store saves the credentials to the macOS OS keyring (via go-keyring).
func Store(liAt, jsessionID string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("keyring access is only supported on macOS (darwin)")
	}

	if err := keyring.Set(keyringService, "li_at", liAt); err != nil {
		return fmt.Errorf("store li_at: %w", err)
	}

	if err := keyring.Set(keyringService, "jsessionid", jsessionID); err != nil {
		return fmt.Errorf("store jsessionid: %w", err)
	}

	return nil
}
